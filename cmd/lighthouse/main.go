package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-oidfed/lib"
	"github.com/lestrrat-go/jwx/v3/jwa"

	"github.com/go-oidfed/lighthouse"
	"github.com/go-oidfed/lighthouse/cmd/lighthouse/config"
)

func main() {
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}
	config.Load(configFile)
	log.Println("Loaded Config")
	c := config.Get()
	initKey()
	log.Println("Loaded signing key")
	for _, tmc := range c.TrustMarks {
		if err := tmc.Verify(
			c.EntityID, c.Endpoints.TrustMarkEndpoint.ValidateURL(c.EntityID),
			oidfed.NewTrustMarkSigner(signingKey, jwa.ES512()),
		); err != nil {
			log.Fatal(err)
		}
	}

	subordinateStorage, trustMarkedEntitiesStorage, err := config.LoadStorageBackends(c)
	if err != nil {
		log.Fatal(err)
	}

	entity, err := lighthouse.NewLightHouse(
		c.EntityID, c.AuthorityHints,
		&oidfed.Metadata{
			FederationEntity: &oidfed.FederationEntityMetadata{
				OrganizationName: c.OrganizationName,
				LogoURI:          c.LogoURI,
			},
		},
		signingKey, jwa.ES512(), c.ConfigurationLifetime, lighthouse.SubordinateStatementsConfig{
			MetadataPolicies:             nil,
			SubordinateStatementLifetime: 3600,
			// TODO read all of this from config or a storage backend
		},
	)
	if err != nil {
		panic(err)
	}

	entity.MetadataPolicies = c.MetadataPolicy
	// TODO other constraints etc.

	entity.TrustMarkIssuers = c.TrustMarkIssuers
	entity.TrustMarkOwners = c.TrustMarkOwners
	entity.TrustMarks = c.TrustMarks

	var trustMarkCheckerMap map[string]lighthouse.EntityChecker
	if len(c.TrustMarkSpecs) > 0 {
		specs := make([]oidfed.TrustMarkSpec, len(c.TrustMarkSpecs))
		for i, s := range c.TrustMarkSpecs {
			specs[i] = s.TrustMarkSpec
			if s.CheckerConfig.Type != "" {
				if trustMarkCheckerMap == nil {
					trustMarkCheckerMap = make(map[string]lighthouse.EntityChecker)
				}
				trustMarkCheckerMap[s.TrustMarkType], err = lighthouse.EntityCheckerFromEntityCheckerConfig(
					s.CheckerConfig,
				)
				if err != nil {
					panic(err)
				}
			}
		}
		entity.TrustMarkIssuer = oidfed.NewTrustMarkIssuer(c.EntityID, entity.GeneralJWTSigner.TrustMarkSigner(), specs)
	}
	log.Println("Initialized Entity")

	if endpoint := c.Endpoints.FetchEndpoint; endpoint.IsSet() {
		entity.AddFetchEndpoint(endpoint, subordinateStorage)
	}
	if endpoint := c.Endpoints.ListEndpoint; endpoint.IsSet() {
		entity.AddSubordinateListingEndpoint(endpoint, subordinateStorage, trustMarkedEntitiesStorage)
	}
	if endpoint := c.Endpoints.ResolveEndpoint; endpoint.IsSet() {
		entity.AddResolveEndpoint(endpoint)
	}
	if endpoint := c.Endpoints.TrustMarkStatusEndpoint; endpoint.IsSet() {
		entity.AddTrustMarkStatusEndpoint(endpoint, trustMarkedEntitiesStorage)
	}
	if endpoint := c.Endpoints.TrustMarkedEntitiesListingEndpoint; endpoint.IsSet() {
		entity.AddTrustMarkedEntitiesListingEndpoint(endpoint, trustMarkedEntitiesStorage)
	}
	if endpoint := c.Endpoints.TrustMarkEndpoint; endpoint.IsSet() {
		entity.AddTrustMarkEndpoint(endpoint, trustMarkedEntitiesStorage, trustMarkCheckerMap)
	}
	if endpoint := c.Endpoints.TrustMarkRequestEndpoint; endpoint.IsSet() {
		entity.AddTrustMarkRequestEndpoint(endpoint, trustMarkedEntitiesStorage)
	}
	if endpoint := c.Endpoints.EnrollmentEndpoint; endpoint.IsSet() {
		var checker lighthouse.EntityChecker
		if checkerConfig := endpoint.CheckerConfig; checkerConfig.Type != "" {
			checker, err = lighthouse.EntityCheckerFromEntityCheckerConfig(checkerConfig)
			if err != nil {
				panic(err)
			}
		}
		entity.AddEnrollEndpoint(endpoint.EndpointConf, subordinateStorage, checker)
	}
	if endpoint := c.Endpoints.EnrollmentRequestEndpoint; endpoint.IsSet() {
		entity.AddEnrollRequestEndpoint(endpoint, subordinateStorage)
	}
	if endpoint := c.Endpoints.EntityCollectionEndpoint; endpoint.IsSet() {
		entity.AddEntityCollectionEndpoint(endpoint)
	}
	log.Println("Added Endpoints")

	log.Printf("Start serving on port %d\n", c.ServerPort)
	if err = http.ListenAndServe(fmt.Sprintf(":%d", c.ServerPort), entity.HttpHandlerFunc()); err != nil {
		panic(err)
	}
	// if err = entity.Listen(fmt.Sprintf(":%d", c.ServerPort)); err != nil {
	// 	panic(err)
	// }

}
