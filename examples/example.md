# Example use of Skiperator Application with Accesserator SecurityConfig

## Token Exchange with Texas

To perform a token exchange with texas, 
you first need an access token issued to an end user. This can be done with
requesting a token from mock-oauth2 server. You can then exchange that access token with 
an access token destined for another app, but with the user context preserved. 

To request an access token from mock-oauth2 server you first have to start `kubefwd` 
with the following command:

```bash
flox activate -- sudo kubefwd svc -n auth
```

You can then request an access token from mock-oauth2 server and place it on your clipboard with the following command:

```bash
curl -s -X POST http://mock-oauth2.auth:8080/accesserator/token \
  -d "grant_type=authorization_code" -d "code=code" -d "client_id=something" \
  | yq -r '.access_token' \
  | tr -d '\n' \
  | { command -v pbcopy >/dev/null && pbcopy || xclip -selection clipboard || xsel --clipboard --input; }
```

To exchange the token the easiest way is to start `k9s` and open up a shell in the pod created by the Skiperator Application `app`.

```bash
flox activate -- k9s
```

You can then exchange the token with the following request (executed in a shell opened on the pod `app-<hash-suffix>` in `k9s`):

```bash
curl -X POST $TEXAS_URL/api/v1/token/exchange \
  -H "Content-Type: application/json" \
  -d '{
    "identity_provider": "tokenx",
    "target": "kind-accesserator:test:another-app",
    "user_token": "<USER_TOKEN>"
  }'
```


## Opa sidecar creation

(in k8s/)
```bash
make clean local deploy
cd example && make example
```

1. Check that "Bundle loaded and activated successfully"
2. Update example.yaml to a new version

(in examples/)
```bash
make example
```

1. Check that "Bundle loaded and activated successfully"