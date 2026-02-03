KIND_CLUSTER_NAME   := skip-local
KUBECONTEXT         := kind-$(KIND_CLUSTER_NAME)



.PHONY: example
example:
	@echo -e "Applying example"
	@kubectl apply -f example.yaml

	@echo -e "🤞 Creating secrets..."
	@kubectl create secret generic opa-github-secret -n test --from-env-file=.github_token.env --dry-run=client -o yaml | kubectl apply -f -
	@kubectl create secret generic opa-sign-secret -n test --from-file=public_sign_key=.public_sign_key.pem --dry-run=client -o yaml | kubectl apply -f -

	@echo -e "✅ Secrets deployed!"