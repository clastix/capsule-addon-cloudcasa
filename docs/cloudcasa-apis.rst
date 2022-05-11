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

Organizations
=============

Every user in CloudCasa belongs to one or more organizations, simply
called Orgs. When a user signs up and logs in for the first time, a
new "default" Org is created. The user is the admin for this Org and
can invite other users (before this can be done, Org name needs to be
changed from "default" to a more specific name). 

All users in an Org can see all resources except as limited by RBAC.

Backup Flow
===========

Cluster admin should register the cluster in CloudCasa (assume for now
that this is done in UI).

When a new tenant is created (say, "acme-dev'):

- Create an empty user group in CloudCasa to be associated with this
  tenant. For simplicity, the name can be tied to tenant name
  (e.g. "usergroup-acme-dev").

  This group can be given the following permissions (in addition to
  any other required ones):

  - policies.create
  - kubebackups.create
  - kubeclusters.backup and kubeclusters.restore for the specific
    cluster registered above.

- Invite the tenant admin user to CloudCasa by creating the
  "orginvite" resource. Pass the user group created above as part of
  the request body for orginvite. This way, the tenant user would
  inherit the permissions already given to the user group.

  The invited users would receive an email from CloudCasa containing
  the link to Sign up or Sign in. Once they do that, they will be able
  to see the cluster and can proceed to define a backup.

Permissions
===========

CloudCasa supports the following list of permissions at the moment
(the list is not exhaustive):

backupinstances.read
    Allows to see recovery points and all related catalog resources. 

kubebackups.create
kubebackups.read
kubebackups.readwrite
    Permissions to control creation of Kubernetes backups. 

kubeclusters.backup
kubeclusters.control
kubeclusters.create
kubeclusters.read
kubeclusters.readwrite
kubeclusters.restore
kubeclusters.scan
    Permissions to control cluster operations.

kubehooks.create
kubehooks.read
kubehooks.readwrite
    Permissions to control operations on application hooks. Hooks can
    be used to freeze/thaw applications during backups and restores.

kubenamespaces.read
    Allows one to see a namespace.

objectstores.create
objectstores.read
objectstores.readwrite
    Permissions to control creation of backup target S3 storage.

policies.create
policies.read
policies.readwrite
    Permissions to control policy operations. Policies allow
    scheduling of backups. 

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
        ],
        "acls": [
            {
                "resource: "allresources",
                "permissions": [
                    "kubebackups.create",
                    "policies.create"
                 ]
            },
            {
                "resource: "kubeclusters",
                "resourceIds": ["624e188b47ea96df7df16c22"],
                "permissions": [
                    "kubeclusters.backup",
                    "kubeclusters.restore"
                 ]
            },
        ]
    }

    201 CREATED
    
    {
        "_id": "624e188b47ea96df6df16c22",
        "name": "testusergroup",
        "users": [
            "624df340e1980b575f252fc7"
        ],
        ...
    }

**Notes**

- "users" is optional.

Schema for "acls"
~~~~~~~~~~~~~~~~~

resource
    Required. Name of the resource type. E.g. "policies", "kubebackups".

resourceIds
    Optional. IDs of the resources for which permissions need to be
    assigned.

permissions
    List of permissions strings. E.g. ["policies.create"]. One of
    "permissions" or "roles" must be provided.

roles
    List of role IDs. One of "permissions" or "roles" must be
    provided.

Updating a user group
---------------------

.. code-block:: javascript

    PUT /api/v1/usergroups

Get a user group
----------------

.. code-block:: javascript

    GET /api/v1/usergroups/624e188b47ea96df6df16c22

List user groups
----------------

.. code-block:: javascript

    GET /api/v1/usergroups

Delete a user group
-------------------

.. code-block:: javascript

    DELETE /api/v1/usergroups/624e188b47ea96df6df16c22

Roles
=====

Roles are group of permissions and can be used to conveniently assign
a group of permissions as a unit.

Creating a role
---------------

.. code-block:: javascript

    POST /api/v1/roles
    
    {
        "name": "testrole",
        "permissions": [
            "kubebackups.create",
            "policies.create"
        ]
    }
    
    201 CREATED

    {
        "name": "testrole",
        "permissions": [
            "kubebackups.create",
            "policies.create"
        ],
        "type": "CUSTOM",
        ...
    }

Updating a role
---------------

.. code-block:: javascript

    PUT /api/v1/roles

Get a role
==========

.. code-block:: javascript

    GET /api/v1/roles/624e188b47ea96df6df16c22

List roles
----------

.. code-block:: javascript

    GET /api/v1/roles

Delete a role
-------------

.. code-block:: javascript

    DELETE /api/v1/roles/624e188b47ea96df6df16c22

**Notes**

- Deletion of roles would fail if they are in use.

Inviting a user to CloudCasa
============================

This is achieved by creating a resource called "orginvite". 

Creating an orginvite
---------------------

.. code-block:: javascript

    POST /api/v1/orginvites

    {
        "email": "testuser@example.com",
        "first_name": "Zaphod",
        "last_name": "B",
        "usergroups": ["624e188b47ea96df6df16c22"]
    }

    201 CREATED

    {
        "_id": "61a500e02b91151e39ec3895",
        ...
    }

Assigning permissions
=====================

Permissions will be mainly assigned and updated from a user, user
group, or api key's point of view.

Assigning permissions on resources to a user::

    POST /api/v1/orgs/625d6e1fa50646661ff9b0d8/user/625d6e1fa50646661ff9b0d6/action/update-acls

    {
        "acls": [
            {
                "roles": [
                    "6252e1ac85a0eec3867a4542",
                    ...
                ],
                "permissions": [
                    "kubeclusters.view",
                    ...
                ],
                "resource": "allresources",
                "resourceIds": [<resource ID1>, ...]
            }
        ]
    }

    200 OK

Permissions can be assigned to a user group or an API key in a similar
way:: 

    POST /api/v1/usergroups/625205940fe403fbe51aacf9/action/update-acls
    POST /api/v1/apikeys/625205940fe403fbe51aacf9/action/update-acls

**Notes**

- resource IDs are optional. When not given, permissions apply to all
  resources of the given type.

- One of "roles" or "permissions" must be given.

- A rule can contain both permissions and roles.

