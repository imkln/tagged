# First-time setup and deployment

Complete this guide before running the app locally with Google sign-in. Install Python 3, AWS CLI, and GitHub CLI. Run commands from the repository root.

## 1. Register Google sign-in

1. Open [Google Cloud Console](https://console.cloud.google.com/). In the project picker at the top, choose **New project**, name it **Tagged**, click **Create**, and select the new project.
2. Open **Google Auth Platform** using the console search. Click **Get started**. Enter **Tagged** as the app name, select your support email, choose **External** as the audience, and enter your contact email. Complete the form and click **Create**.
3. Open **Google Auth Platform → Data Access → Add or remove scopes**. Select `openid`, `.../auth/userinfo.email`, and `.../auth/userinfo.profile`. Click **Update**, then **Save**.
4. Open **Google Auth Platform → Clients → Create client** and fill in:

   | Field | Value |
   | --- | --- |
   | Application type | Web application |
   | Name | Tagged |
   | Authorized JavaScript origins → Add URI | `https://<prefix>.auth.<region>.amazoncognito.com` |
   | Authorized redirect URIs → Add URI | `https://<prefix>.auth.<region>.amazoncognito.com/oauth2/idpresponse` |

   Replace `<region>` with your AWS region and `<prefix>` with a unique Cognito domain prefix, for example `tagged-<aws-account-id>`. Use the same prefix for `COGNITO_DOMAIN_PREFIX` below, then click **Create**.
5. Copy the displayed **Client ID** and **Client secret** for the GitHub settings in section 3.

Reference: [AWS Google sign-in setup](https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-user-pools-social-idp-google.html).

## 2. Bootstrap AWS access

With AWS CLI and GitHub CLI authenticated, run:

```sh
python3 script/bootstrap.py
```

The script creates the state bucket, OIDC provider, and deployment role, then creates or updates the four repository variables listed below.

## 3. Configure GitHub

1. Open the repository on GitHub, then **Settings → Secrets and variables → Actions → Variables**.
2. Confirm the variables set by bootstrap are listed:

   | Name | Set by bootstrap |
   | --- | --- |
   | `AWS_REGION` | Bootstrap region |
   | `AWS_ROLE_ARN` | Deployment role ARN |
   | `TF_STATE_BUCKET` | Terraform state bucket name |
   | `TF_STATE_KEY` | Terraform state object key |

3. Click **New repository variable** to add the remaining entries:

   | Name | Value |
   | --- | --- |
   | `GOOGLE_CLIENT_ID` | Client ID copied from Google |
   | `COGNITO_DOMAIN_PREFIX` | The text replacing `<prefix>` in both Google URLs |

4. Switch to **Secrets** and click **New repository secret** for each entry. Bootstrap does not set these:

   | Name | Value |
   | --- | --- |
   | `GOOGLE_CLIENT_SECRET` | Client secret copied from Google |
   | `OWNER_EMAIL` | Google email allowed to access Tagged |

## 4. Deploy from GitHub Actions

1. Open the repository's **Actions → Deploy → Run workflow**. Select the branch to deploy, check **Apply the Terraform plan**, and click **Run workflow**.
2. Open the successful run, expand **terraform output website_url**, and open the displayed URL.
