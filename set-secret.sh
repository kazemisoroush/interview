#!/usr/bin/env bash
# One-time (and rotation) setup: mint a subscription token, pick a passphrase, write
# both into the secret, and force the function to pick them up.
#
#   ./set-secret.sh
#
# The token never reaches the shell history, the script's output, or any file. The
# forced cold start at the end is not optional: the handler caches the secret in
# `booted`, so a warm container keeps serving the old value and every request keeps
# coming back 401 long after the secret is correct.
set -euo pipefail

REGION=us-east-1
STACK=InterviewStack

out() { aws cloudformation describe-stacks --region "$REGION" --stack-name "$STACK" \
          --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text; }

SECRET=$(out ProviderSecretArn)
APP=$(out AppUrl)
FN=$(aws cloudformation describe-stack-resources --region "$REGION" --stack-name "$STACK" \
       --query 'StackResources[?ResourceType==`AWS::Lambda::Function`].PhysicalResourceId' --output text)

echo "==> claude setup-token (a browser will open; finish the login there)"
TOKEN=$(claude setup-token | grep -oE 'sk-ant-[A-Za-z0-9_-]+' | tail -1)
[ -n "$TOKEN" ] || { echo "no token found in the setup-token output" >&2; exit 1; }

# Generated here rather than invented, so it is long enough to be worth the gate.
PASS=$(openssl rand -hex 12)

aws secretsmanager put-secret-value --region "$REGION" --secret-id "$SECRET" \
  --secret-string "$(printf '{"CLAUDE_CODE_OAUTH_TOKEN":"%s","PASSPHRASE":"%s"}' "$TOKEN" "$PASS")" \
  >/dev/null

# Any configuration change replaces the running containers, which is the only way to
# drop the cached secret without waiting for the old ones to age out.
aws lambda update-function-configuration --region "$REGION" --function-name "$FN" \
  --environment "Variables={PROVIDER_SECRET_ARN=$SECRET,SECRET_SET_AT=$(date +%s)}" >/dev/null
aws lambda wait function-updated --region "$REGION" --function-name "$FN"

echo
echo "open this, once, on the phone you will interview with:"
echo "${APP}#pass=${PASS}"
