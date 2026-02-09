# API Reference

Packages:

- [accesserator.kartverket.no/v1alpha](#accesseratorkartverketnov1alpha)

# accesserator.kartverket.no/v1alpha

Resource Types:

- [SecurityConfig](#securityconfig)




## SecurityConfig
<sup><sup>[↩ Parent](#accesseratorkartverketnov1alpha )</sup></sup>






SecurityConfig is the Schema for the securityconfigs API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>accesserator.kartverket.no/v1alpha</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>SecurityConfig</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigspec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of SecurityConfig<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigstatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of SecurityConfig<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### SecurityConfig.spec
<sup><sup>[↩ Parent](#securityconfig)</sup></sup>



spec defines the desired state of SecurityConfig

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>applicationRef</b></td>
        <td>string</td>
        <td>
          ApplicationRef is a reference to the name of the SKIP application for which this SecurityConfig applies.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigspectokenx">tokenx</a></b></td>
        <td>object</td>
        <td>
          Tokenx indicates whether a sidecar (called Texas) is started with the application referred to by `applicationRef`
that provides an endpoint which is available to the application on the env var TEXAS_URL.
The endpoint conforms to the OAuth 2.0 Token Exchange standard (RFC 8693).
accessPolicies in the Application manifest of the application referred to by applicationRef
will be used to restrict which applications can exchange tokens where the specified application is the intended audience.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### SecurityConfig.spec.tokenx
<sup><sup>[↩ Parent](#securityconfigspec)</sup></sup>



Tokenx indicates whether a sidecar (called Texas) is started with the application referred to by `applicationRef`
that provides an endpoint which is available to the application on the env var TEXAS_URL.
The endpoint conforms to the OAuth 2.0 Token Exchange standard (RFC 8693).
accessPolicies in the Application manifest of the application referred to by applicationRef
will be used to restrict which applications can exchange tokens where the specified application is the intended audience.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          Enabled indicates whether the TokenX sidecar should be included for the application.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### SecurityConfig.status
<sup><sup>[↩ Parent](#securityconfig)</sup></sup>



status defines the observed state of SecurityConfig

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>ready</b></td>
        <td>boolean</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#securityconfigstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>phase</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### SecurityConfig.status.conditions[index]
<sup><sup>[↩ Parent](#securityconfigstatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>