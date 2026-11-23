package storage

import (
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

func TestIssuedTrustMarkInstances_ListCountRevoke(t *testing.T) {
	t.Parallel()
	store := newTrustMarkOwnerTestStorage(t)
	instances := NewIssuedTrustMarkInstanceStorage(store.db)
	const (
		now   = int64(1_000_000)
		typeA = "https://tm.example.org/a"
		typeB = "https://tm.example.org/b"
		one   = "https://one.example.org"
		two   = "https://two.example.org"
	)
	for _, inst := range []model.IssuedTrustMarkInstance{
		{JTI: "a-one-active", TrustMarkType: typeA, Subject: one, ExpiresAt: int(now + 60)},
		{JTI: "a-one-forever", TrustMarkType: typeA, Subject: one},
		{JTI: "a-two-expired", TrustMarkType: typeA, Subject: two, ExpiresAt: int(now - 60)},
		{JTI: "b-one-revoked", TrustMarkType: typeB, Subject: one, ExpiresAt: int(now + 60), Revoked: true},
		{JTI: "b-two-revoked-expired", TrustMarkType: typeB, Subject: two, ExpiresAt: int(now - 60), Revoked: true},
	} {
		require.NoError(t, instances.Create(&inst))
	}
	jtis := func(q model.InstanceQuery) []string {
		q.Now = now
		got, _, err := instances.List(q)
		require.NoError(t, err)
		var out []string
		for _, i := range got {
			out = append(out, i.JTI)
		}
		return out
	}

	assert.ElementsMatch(
		t, []string{"a-one-active", "a-one-forever", "a-two-expired", "b-one-revoked", "b-two-revoked-expired"},
		jtis(model.InstanceQuery{}),
	)
	assert.ElementsMatch(
		t, []string{"a-one-active", "a-one-forever", "a-two-expired"}, jtis(model.InstanceQuery{TrustMarkType: typeA}),
	)
	assert.ElementsMatch(
		t, []string{"a-one-active", "a-one-forever", "b-one-revoked"}, jtis(model.InstanceQuery{Subject: one}),
	)
	assert.ElementsMatch(
		t, []string{"a-one-active", "a-one-forever"}, jtis(model.InstanceQuery{State: model.TrustMarkStatusActive}),
	)
	assert.ElementsMatch(t, []string{"a-two-expired"}, jtis(model.InstanceQuery{State: model.TrustMarkStatusExpired}))
	assert.ElementsMatch(
		t, []string{"b-one-revoked", "b-two-revoked-expired"},
		jtis(model.InstanceQuery{State: model.TrustMarkStatusRevoked}),
	)

	got, total, err := instances.List(model.InstanceQuery{Now: now, Limit: 2, Offset: 4})
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, int64(5), total)

	active, revoked, err := instances.CountByState(now)
	require.NoError(t, err)
	assert.Equal(t, int64(2), active)
	assert.Equal(t, int64(2), revoked)

	n, err := instances.RevokeByTypeAndSubject(typeA, one)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	n, err = instances.RevokeByTypeAndSubject(typeA, one)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.ElementsMatch(t, []string{"a-two-expired"}, jtis(model.InstanceQuery{State: model.TrustMarkStatusExpired}))
}

// subjectWithInstances creates a spec, an active subject and two live
// instances: one linked to the subject and one of the spec's type issued to
// its entity before the subject existed (unlinked).
func subjectWithInstances(t *testing.T, store *Storage) (specID, subjectID string) {
	t.Helper()
	specs := store.TrustMarkSpecStorage()
	spec, err := specs.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-a"})
	require.NoError(t, err)
	subject, err := specs.CreateSubject(
		"type-a", &model.AddTrustMarkSubject{EntityID: "https://a.example.org", Status: model.StatusActive},
	)
	require.NoError(t, err)
	instances := NewIssuedTrustMarkInstanceStorage(store.db)
	require.NoError(
		t, instances.Create(
			&model.IssuedTrustMarkInstance{
				JTI: "linked", TrustMarkType: "type-a", Subject: "https://a.example.org", TrustMarkSubjectID: subject.ID,
			},
		),
	)
	require.NoError(
		t, instances.Create(
			&model.IssuedTrustMarkInstance{JTI: "unlinked", TrustMarkType: "type-a", Subject: "https://a.example.org"},
		),
	)
	return strconv.FormatUint(uint64(spec.ID), 10), strconv.FormatUint(uint64(subject.ID), 10)
}

func revokedJTIs(t *testing.T, store *Storage) []string {
	t.Helper()
	got, _, err := NewIssuedTrustMarkInstanceStorage(store.db).List(
		model.InstanceQuery{State: model.TrustMarkStatusRevoked},
	)
	require.NoError(t, err)
	var out []string
	for _, i := range got {
		out = append(out, i.JTI)
	}
	return out
}

func TestTrustMarkSubject_LeavingActiveRevokes(t *testing.T) {
	t.Parallel()
	tests := map[string]func(specs *TrustMarkSpecStorage, specID, subjectID string) error{
		"transition to pending": func(specs *TrustMarkSpecStorage, specID, subjectID string) error {
			changed, err := specs.TransitionSubjectStatus(specID, subjectID, model.StatusActive, model.StatusPending)
			if err == nil && !changed {
				err = errors.New("not changed")
			}
			return err
		},
		"change to blocked": func(specs *TrustMarkSpecStorage, specID, subjectID string) error {
			_, err := specs.ChangeSubjectStatus(specID, subjectID, model.StatusBlocked)
			return err
		},
		"update to inactive": func(specs *TrustMarkSpecStorage, specID, subjectID string) error {
			_, err := specs.UpdateSubject(
				specID, subjectID, &model.AddTrustMarkSubject{EntityID: "https://a.example.org", Status: model.StatusInactive},
			)
			return err
		},
		"update entity ID": func(specs *TrustMarkSpecStorage, specID, subjectID string) error {
			_, err := specs.UpdateSubject(
				specID, subjectID, &model.AddTrustMarkSubject{EntityID: "https://b.example.org", Status: model.StatusActive},
			)
			return err
		},
		"delete subject": func(specs *TrustMarkSpecStorage, specID, subjectID string) error {
			return specs.DeleteSubject(specID, subjectID)
		},
		"delete spec": func(specs *TrustMarkSpecStorage, specID, _ string) error {
			return specs.Delete(specID)
		},
	}
	for name, change := range tests {
		t.Run(
			name, func(t *testing.T) {
				t.Parallel()
				store := newTrustMarkOwnerTestStorage(t)
				specID, subjectID := subjectWithInstances(t, store)
				require.NoError(t, change(store.TrustMarkSpecStorage(), specID, subjectID))
				assert.ElementsMatch(t, []string{"linked", "unlinked"}, revokedJTIs(t, store))
			},
		)
	}
}

// TestTransitionSubjectStatus_RevocationRollsBack checks that the revocation
// is part of the caller's transaction: rolling it back keeps both the status
// and the instances.
func TestTransitionSubjectStatus_RevocationRollsBack(t *testing.T) {
	t.Parallel()
	store := newTrustMarkOwnerTestStorage(t)
	specID, subjectID := subjectWithInstances(t, store)
	errRollback := errors.New("roll back")
	err := store.db.Transaction(
		func(tx *gorm.DB) error {
			specs := &TrustMarkSpecStorage{db: tx}
			changed, err := specs.TransitionSubjectStatus(specID, subjectID, model.StatusActive, model.StatusBlocked)
			require.NoError(t, err)
			require.True(t, changed)
			return errRollback
		},
	)
	require.ErrorIs(t, err, errRollback)
	assert.Empty(t, revokedJTIs(t, store))
	subject, err := store.TrustMarkSpecStorage().GetSubject(specID, subjectID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusActive, subject.Status)
}

// TestTransitionSubjectStatus_ByEntityAndType addresses the spec by its type
// and the subject by its entity ID, as the numeric IDs are.
func TestTransitionSubjectStatus_ByEntityAndType(t *testing.T) {
	t.Parallel()
	store := newTrustMarkOwnerTestStorage(t)
	subjectWithInstances(t, store)
	specs := store.TrustMarkSpecStorage()
	changed, err := specs.TransitionSubjectStatus("type-a", "https://a.example.org", model.StatusActive, model.StatusInactive)
	require.NoError(t, err)
	assert.True(t, changed)
	changed, err = specs.TransitionSubjectStatus("type-b", "https://a.example.org", model.StatusInactive, model.StatusActive)
	require.NoError(t, err)
	assert.False(t, changed)
}

// TestLockSpecAndSubject checks that the lock UPDATEs change nothing and
// accept unknown rows.
func TestLockSpecAndSubject(t *testing.T) {
	t.Parallel()
	store := newTrustMarkOwnerTestStorage(t)
	specID, subjectID := subjectWithInstances(t, store)
	specs := store.TrustMarkSpecStorage()
	beforeSpec, err := specs.Get(specID)
	require.NoError(t, err)
	beforeSubject, err := specs.GetSubject(specID, subjectID)
	require.NoError(t, err)

	err = store.db.Transaction(
		func(tx *gorm.DB) error {
			ts := &TrustMarkSpecStorage{db: tx}
			require.NoError(t, ts.LockSpec(specID))
			require.NoError(t, ts.LockSpec("type-a"))
			require.NoError(t, ts.LockSubject(specID, subjectID))
			require.NoError(t, ts.LockSubject("type-a", "https://a.example.org"))
			require.NoError(t, ts.LockSpec("unknown"))
			require.NoError(t, ts.LockSubject(specID, "999999"))
			return nil
		},
	)
	require.NoError(t, err)
	afterSpec, err := specs.Get(specID)
	require.NoError(t, err)
	afterSubject, err := specs.GetSubject(specID, subjectID)
	require.NoError(t, err)
	assert.Equal(t, beforeSpec.UpdatedAt, afterSpec.UpdatedAt)
	assert.Equal(t, *beforeSubject, *afterSubject)
}

// TestTrustMarkSpecStorageWritesConcurrent runs the storage writes that use a
// transaction of their own, without an outer one (as lhmigrate, lhsetup and
// the additional-claims routes call them), overlapping on file-backed SQLite:
// they serialize and none fails with "database is locked".
func TestTrustMarkSpecStorageWritesConcurrent(t *testing.T) {
	t.Parallel()
	store, err := NewStorage(Config{Driver: DriverSQLite, DataDir: t.TempDir()})
	require.NoError(t, err)
	specs := store.TrustMarkSpecStorage()
	for round := range 10 {
		trustMarkType := "type-" + strconv.Itoa(round)
		_, err = specs.Create(&model.AddTrustMarkSpec{TrustMarkType: trustMarkType})
		require.NoError(t, err)
		for _, e := range []string{"a", "b"} {
			_, err = specs.CreateSubject(
				trustMarkType, &model.AddTrustMarkSubject{EntityID: "https://" + e + ".example.org", Status: model.StatusActive},
			)
			require.NoError(t, err)
		}
		writes := []func() error{
			func() error {
				_, err := specs.ChangeSubjectStatus(trustMarkType, "https://a.example.org", model.StatusBlocked)
				return err
			},
			func() error {
				_, err := specs.ChangeSubjectStatus(trustMarkType, "https://a.example.org", model.StatusInactive)
				return err
			},
			func() error {
				_, err := specs.UpdateSubject(
					trustMarkType, "https://b.example.org",
					&model.AddTrustMarkSubject{EntityID: "https://b.example.org", Status: model.StatusPending},
				)
				return err
			},
			func() error { return specs.DeleteSubject(trustMarkType, "https://b.example.org") },
		}
		errs := make([]error, len(writes))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i, write := range writes {
			wg.Go(
				func() {
					<-start
					errs[i] = write()
				},
			)
		}
		close(start)
		wg.Wait()
		for i, err := range errs {
			// The update may find the subject the delete removed.
			if _, notFound := errors.AsType[model.NotFoundError](err); err != nil && !notFound {
				t.Fatalf("round %d: write %d: %v", round, i, err)
			}
		}
		require.NoError(t, specs.Delete(trustMarkType))
	}
}
