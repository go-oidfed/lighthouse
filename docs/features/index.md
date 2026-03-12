# Overview of Supported and Planned Features

## Endpoints

- [X] Entity Configuration Endpoint
- [X] Subordinate Listing Endpoint
- [X] Fetching Endpoint
- [X] Resolve Endpoint
- [X] Trust Mark Endpoint
- [X] Trust Marked Entities Listing Endpoint                                
- [X] Trust Mark Status Endpoint   
- [X] Federation Historical Keys Endpoint
- [X] Endpoint to automatically enroll entities
- [X] Endpoint to request enrollment
- [X] Endpoint to request to be entitled for a trust mark
- [X] Entity Collection Endpoint

## Entity Configuration

- [X] Create and publish Entity Configuration
- [X] Set Authority Hints
- [X] Automatically refresh trust marks in Entity Configuration
- [X] Support for publishing "external" keys in `jwks`
- [X] Configurable Federation Entity Metadata
- [X] Support additional Claims in Entity Configuration

## Federation

- [X] Configure Trust Mark Issuers
- [X] Configure Trust Mark Owners
- [X] General Metadata Policies for all Entities
- [X] Support for individual Metadata Policies per Subordinate
- [X] Support for Custom Metadata Policy Operators including marking 
  critical operators
- [X] General Constraints for all Entities
- [X] Support for individual Constraints per Subordinate

## Subordinates

- [X] Management of Subordinates
  - [X] Full CRUD support
- [X] Support for individual Metadata Policies per Subordinate
- [X] Support for individual Constraints per Subordinate
- [X] Support for individual Metadata overwrite per Subordinate
- [ ] Automatic updates of Subordinate JWKS (for key rotation)

## Trust Marks
### Trust Mark Issuance

- [X] Issuance of Trust Marks
- [X] Support for Trust Mark Delegation
- [X] Automatic, configurable Checks for Trust Mark Issuance
- [X] Manual management of Trust Mark Subjects
- [X] Additional Trust Mark Claims
- [X] Additional Trust Mark Claims per Subject

### Trust Mark Verification

- [X] Trust Mark JWT Verification for non-delegated Trust Marks           
- [X] Trust Mark JWT Verification for Trust Marks using delegation
- [ ] Trust Mark Verification using the Trust Mark Status Endpoint       

## Enrollment

- [X] Endpoint to automatically enroll entities
  - [X] Automatic, configurable Checks for Enrollment
- [X] Endpoint to request enrollment

## Signing

- [X] Support of various signing algorithms
- [X] Support for Automatic Key Rotation
- [X] Support for pkcs11
- [X] Support for publishing "external" keys

## Trust Evaluation
- [X] Collect and build Trust Chain
- [X] Verify Trust Chains
- [X] Evaluating Constraints
- [X] Resolve Metadata
- [X] Applying Metadata Policies
- [X] Applying Metadata from Superiors
- [X] Trust Evaluation via Resolve Endpoint


## Technical

- [X] Endpoints supporting GET requests
- [ ] Endpoints supporting POST requests
- [ ] Endpoints supporting Client Authentication
- [X] JWT Type Verification

## Statistics

- [X] Capture request metrics (timing, status, errors)
- [X] Client tracking (IP, User-Agent, country via GeoIP)
- [X] Query parameter tracking
- [X] REST API for statistics queries
- [X] CLI commands for statistics
- [X] CSV/JSON export
- [X] Automatic daily aggregation
- [X] Configurable data retention