package storage

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-oidfed/lighthouse/storage/model"
)

func TestTransitionStatusByDBID(t *testing.T) {
	t.Parallel()
	subs := newTrustMarkOwnerTestStorage(t).SubordinateStorage()
	require.NoError(
		t, subs.Add(
			model.ExtendedSubordinateInfo{
				BasicSubordinateInfo: model.BasicSubordinateInfo{
					EntityID: "https://sub.example.org", Status: model.StatusPending,
				},
			},
		),
	)
	info, err := subs.Get("https://sub.example.org")
	require.NoError(t, err)
	id := strconv.FormatUint(uint64(info.ID), 10)

	changed, err := subs.TransitionStatusByDBID(id, model.StatusPending, model.StatusActive)
	require.NoError(t, err)
	require.True(t, changed)

	// The status is no longer the one expected: nothing changes.
	changed, err = subs.TransitionStatusByDBID(id, model.StatusPending, model.StatusBlocked)
	require.NoError(t, err)
	require.False(t, changed)
	info, err = subs.Get("https://sub.example.org")
	require.NoError(t, err)
	require.Equal(t, model.StatusActive, info.Status)

	changed, err = subs.TransitionStatusByDBID("999999", model.StatusPending, model.StatusActive)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestTransitionSubjectStatus(t *testing.T) {
	t.Parallel()
	specs := newTrustMarkOwnerTestStorage(t).TrustMarkSpecStorage()
	spec, err := specs.Create(&model.AddTrustMarkSpec{TrustMarkType: "type-a"})
	require.NoError(t, err)
	specID := strconv.FormatUint(uint64(spec.ID), 10)
	subject, err := specs.CreateSubject(
		specID, &model.AddTrustMarkSubject{EntityID: "https://a.example.org", Status: model.StatusPending},
	)
	require.NoError(t, err)
	subjectID := strconv.FormatUint(uint64(subject.ID), 10)

	changed, err := specs.TransitionSubjectStatus(specID, subjectID, model.StatusPending, model.StatusActive)
	require.NoError(t, err)
	require.True(t, changed)

	changed, err = specs.TransitionSubjectStatus(specID, subjectID, model.StatusPending, model.StatusBlocked)
	require.NoError(t, err)
	require.False(t, changed)
	got, err := specs.GetSubject(specID, subjectID)
	require.NoError(t, err)
	require.Equal(t, model.StatusActive, got.Status)

	changed, err = specs.TransitionSubjectStatus(specID, "999999", model.StatusActive, model.StatusBlocked)
	require.NoError(t, err)
	require.False(t, changed)

	changed, err = specs.TransitionSubjectStatus(specID, subjectID, model.StatusActive, model.StatusBlocked)
	require.NoError(t, err)
	require.True(t, changed)
}
