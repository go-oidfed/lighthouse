# Overview of Supported and Planned Features

- [X] Create and publish Entity Configuration                                
- [X] Trust Chains
    - [X] Collect and build Trust Chain
    - [X] Verify Trust Chains
    - [X] Evaluating Constraints
    - [X] Resolve Metadata
        - [X] Applying Metadata Policies  
        - [ ] Applying Metadata from Superiors  
        - [X] Support for Custom Metadata Policy Operators 
    - [X] Resolve Endpoint
- [X] Configure Trust
    - [X] Configure Trust Anchors  
    - [X] Set Authority Hints     
- [X] Endpoints
    - [X] Subordinate Listing Endpoint
    - [X] Fetching Endpoint
    - [X] Resolve Endpoint
    - [X] Trust Mark Endpoint
    - [X] Trust Marked Entities Listing Endpoint                                
    - [ ] Trust Mark Status Endpoint   
    - [ ] Federation Historical Keys Endpoint
    - [X] Endpoint to automatically enroll entities
    - [X] Endpoint to request enrollment
    - [X] Endpoint to request to be entitled for a trust mark
    - [X] Entity Collection Endpoint
- [X] Trust Marks
    - [X] Issuance of Trust Marks
    - [X] Support for Trust Mark Delegation                               
    - [X] Trust Mark JWT Verification for non-delegated Trust Marks           
    - [X] Trust Mark JWT Verification for Trust Marks using delegation
    - [ ] Trust Mark Verification using the Trust Mark Status Endpoint       
- [X] JWT Type Verification   
- [X] Endpoints supporting GET requests
- [ ] Endpoints supporting POST requests
- [ ] Endpoints supporting Client Authentication   
- [ ] Automatic Key Rollover                      
- [X] Entity Checks
    - [X] Automatic, configurable Checks for Enrollment                        
    - [X] Automatic, configurable Checks for Trust Mark Issuance  
- [X] Automatically refresh trust marks in Entity Configuration  
- [ ] Support for multiple signing keys
