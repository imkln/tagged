# First-time setup

Application deployments run through GitHub Actions. Local AWS access is only needed to bootstrap the state bucket and GitHub OIDC access.

## Google sign-in

1. Create a project in [Google Cloud Console](https://console.cloud.google.com/) and configure **Google Auth Platform** for Tagged. Choose an External audience and add your account as a test user.
2. Request only OpenID, email, and profile scopes.
3. Choose a unique Cognito domain prefix. Create a Web application OAuth client with these URLs:
   - Origin: `https://<prefix>.auth.us-east-1.amazoncognito.com`
   - Redirect: `https://<prefix>.auth.us-east-1.amazoncognito.com/oauth2/idpresponse`
4. Save the client ID and secret. Use the same prefix in the GitHub configuration below.

Google provides sign-in only; application hosting remains on AWS. See the [AWS setup guide](https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-user-pools-social-idp-google.html) for console details.

## AWS bootstrap

With AWS CLI and GitHub CLI logged in, run from the repository root:

```sh
python3 script/bootstrap.py
```

The script creates the Terraform state bucket, GitHub OIDC provider, and deployment role. It creates or updates repository variables `AWS_REGION`, `AWS_ROLE_ARN`, `TF_STATE_BUCKET`, and `TF_STATE_KEY`. Access is restricted to workflows on `main`.

Defaults use the current account, repository, and `us-east-1`. Use `--state-bucket`, `--role-name`, `--region`, or `--repo` to override them. Rerunning updates the existing setup.

## GitHub configuration

Under **Settings → Secrets and variables → Actions**, add:

| Repository variable | Value |
| --- | --- |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `COGNITO_DOMAIN_PREFIX` | Prefix registered above |

| Repository secret | Value |
| --- | --- |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `OWNER_EMAIL` | Authorized Google email |
| `OWNER_SUBJECT` | Optional Cognito subject |

## Deploy

After merging, open **Actions → Deploy → Run workflow** on `main`. Leave **apply** unchecked to review the plan; select it when ready to deploy. The website URL appears in the workflow output.
