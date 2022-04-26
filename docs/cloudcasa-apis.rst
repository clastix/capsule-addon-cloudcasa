================
 CloudCasa APIs
================

This document describes CloudCasa APIs that can be used to backup and
restore data in Kubernetes clusters.

Overview
========

- All APIs would be rooted at ``<DOMAIN>/api/v1``. DOMAIN values are: 

  - Staging: "https://api.staging.cloudcasa.io"
  - Production: "https://api.cloudcasa.io"

- All resources have "name" field and the value must be unique.

- Most, if not all, resources support tags which are key/value
  pairs.

- Every created resource will have "_id" field. Do not set this field
  in the request body while creating a resource.

- Some resources also have a "status" field  which is used by the
  backend to describe the current status of the resource. Clients
  should not set this field in input.

- All requests should set "Content-Type" and "Accept" headers to
  "application/json".

OpenAPI Spec
============

OpenAPI V3 spec can be obtained by doing::

    GET /api-docs

Concurrency Control
===================

API responses for every resource include a ETag header (and also a
``_etag`` field) which is a hash value representing the current state
of the resource on the server. Clients are not allowed to edit (PATCH
or PUT) a resource unless they provide an up-to-date ETag for the
resource they are attempting to edit. This prevents overwriting items
with obsolete versions. 

To modify a resource, "If-Match" header must be sent. Here
is an example:

.. code-block::

  If-Match: 00fa3c48c58af86be8222514f2d6452a6a4a6949

If the header is not sent or if invalid value is sent, PATCH and
DELETE would fail. Here are sample errors:

.. code-block:: json

   {
       "_error": {
           "code": 428,
           "message": "To edit a document its etag must be provided using the If-Match header"
       },
       "_status": "ERR"
   }

   {
       "_error": {
           "code": 412,
           "message": "Client and server etags don't match"
       },
       "_status": "ERR"
   }

Authentication
==============

Login to CloudCasa UI and create an API token which needs to be sent
in the header ``Authorization`` as a "Bearer" token, like so::

    Authorization: Bearer <TOKEN>

Backup Flow
===========

- Cluster admin registers the cluster in CloudCasa (assume for now
  that this is done in UI).

- Tenant creation 

  - Cluster admin creates an empty user group in CloudCasa to be
    associated with this tenant.

User Groups
===========

Create a user group
-------------------

.. code-block:: javascript

    POST /api/v1/usergroups
    
    {
        "name": "testusergroup",
        "users": [
            "624df340e1980b575f252fc7"
        ]
    }

    201 CREATED
    
    {
        "_id": "624e188b47ea96df6df16c22",
        "name": "testusergroup",
        "users": [
            "624df340e1980b575f252fc7"
        ]
    }

**Notes**

- "users" is optional.

Updating a user group
---------------------

.. code-block:: javascript

    PUT /api/v1/usergroups

Get a user group
----------------

.. code-block:: javascript

    PUT /api/v1/usergroups/624e188b47ea96df6df16c22

List user groups
----------------

.. code-block:: javascript

    GET /api/v1/usergroups

Delete a user group
-------------------

.. code-block:: javascript

    DELETE /api/v1/usergroups


