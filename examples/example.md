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
    "target": "kind-accesserator:test:yet-another-app",
    "user_token": "eyJraWQiOiJhY2Nlc3NlcmF0b3IiLCJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJzb21lLXJhbmRvbS11dWlkIiwiYXVkIjoic29tZS1yYW5kb20tYXVkaWVuY2UiLCJuYmYiOjE3NjkwOTgyMDIsInJvbGUiOiJzb21lLXJhbmRvbS1yb2xlIiwiaXNzIjoiaHR0cDovL21vY2stb2F1dGgyLmF1dGg6ODA4MC9hY2Nlc3NlcmF0b3IiLCJleHAiOjE3NjkxMDE4MDIsImlhdCI6MTc2OTA5ODIwMiwianRpIjoiMjg5Zjc5ODItZWMzNi00MWMzLThlODYtMjdmMjIxMDhkZjQyIn0.h4F81VU0WYATcFo9-ZuAmLJRhzLxb-SxidC36pjh6CU3EwgdmXA5gtrYqNJ0OD2J420S5DP4na0UW-0hdXmJw61mhLvhVweLXW4Fy18Zzny7pPU1kKLiGZipld_BkvHnnG2tMYi0kNAygf36XXgAh4ENMTh-4AtPITYvHHfvpQCyd-KGqi-5uggCu4qjsBj-ynXKduuFECnZ65Ld1WITJb9IPqwskxL0GU4ccWGRX7vVDKuWJoubLfXZcC-4fWorcwi03YJCn-7GJnhu7tewLoqkLVcrjcnn71aAn40v2bDRmhRiaAwSWAWpIdNcI9eiedL7Rq0CJYFRYCihXV_wzA"
  }'
```
