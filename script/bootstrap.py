import argparse
import json
import os
import subprocess
import sys
from pathlib import Path
from string import Template

ROOT = Path(__file__).resolve().parent.parent


def run(*command, missing=()):
    result = subprocess.run(command, check=False, capture_output=True, text=True)
    if result.returncode and any(code in result.stderr for code in missing):
        return None
    result.check_returncode()
    return result.stdout.strip()


def configure_bucket(aws, bucket, account, region):
    existing = run(
        *aws,
        "s3api",
        "head-bucket",
        "--bucket",
        bucket,
        "--expected-bucket-owner",
        account,
        missing=("(404)", "(NotFound)"),
    )
    if existing is None:
        command = [*aws, "s3api", "create-bucket", "--bucket", bucket]
        if region != "us-east-1":
            command.extend(
                [
                    "--create-bucket-configuration",
                    json.dumps({"LocationConstraint": region}),
                ]
            )
        run(*command)

    run(
        *aws,
        "s3api",
        "put-bucket-versioning",
        "--bucket",
        bucket,
        "--versioning-configuration",
        "Status=Enabled",
    )
    run(
        *aws,
        "s3api",
        "put-public-access-block",
        "--bucket",
        bucket,
        "--public-access-block-configuration",
        json.dumps(
            {
                "BlockPublicAcls": True,
                "BlockPublicPolicy": True,
                "IgnorePublicAcls": True,
                "RestrictPublicBuckets": True,
            }
        ),
    )
    run(
        *aws,
        "s3api",
        "put-bucket-encryption",
        "--bucket",
        bucket,
        "--server-side-encryption-configuration",
        json.dumps(
            {
                "Rules": [
                    {"ApplyServerSideEncryptionByDefault": {"SSEAlgorithm": "AES256"}}
                ],
            }
        ),
    )


def configure_provider(aws, provider_arn):
    existing = run(
        *aws,
        "iam",
        "get-open-id-connect-provider",
        "--open-id-connect-provider-arn",
        provider_arn,
        missing=("(NoSuchEntity)",),
    )
    if existing is None:
        run(
            *aws,
            "iam",
            "create-open-id-connect-provider",
            "--url",
            "https://token.actions.githubusercontent.com",
            "--client-id-list",
            "sts.amazonaws.com",
        )
    elif "sts.amazonaws.com" not in json.loads(existing)["ClientIDList"]:
        run(
            *aws,
            "iam",
            "add-client-id-to-open-id-connect-provider",
            "--open-id-connect-provider-arn",
            provider_arn,
            "--client-id",
            "sts.amazonaws.com",
        )


def configure_role(aws, role_name, provider_arn, subject, policy):
    trust = {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {"Federated": provider_arn},
                "Action": "sts:AssumeRoleWithWebIdentity",
                "Condition": {
                    "StringEquals": {
                        "token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
                        "token.actions.githubusercontent.com:sub": subject,
                    },
                },
            }
        ],
    }
    existing = run(
        *aws,
        "iam",
        "get-role",
        "--role-name",
        role_name,
        missing=("(NoSuchEntity)",),
    )
    if existing is None:
        run(
            *aws,
            "iam",
            "create-role",
            "--role-name",
            role_name,
            "--assume-role-policy-document",
            json.dumps(trust),
            "--tags",
            "Key=Project,Value=tagged",
        )
    else:
        tags = json.loads(existing)["Role"].get("Tags", [])
        if {"Key": "Project", "Value": "tagged"} not in tags:
            raise RuntimeError(
                f"Role {role_name} exists outside this bootstrap; choose another name."
            )
        run(
            *aws,
            "iam",
            "update-assume-role-policy",
            "--role-name",
            role_name,
            "--policy-document",
            json.dumps(trust),
        )
    run(
        *aws,
        "iam",
        "put-role-policy",
        "--role-name",
        role_name,
        "--policy-name",
        "tagged-deployment",
        "--policy-document",
        json.dumps(policy),
    )


def render_policy(value, variables):
    if isinstance(value, dict):
        return {key: render_policy(item, variables) for key, item in value.items()}
    if isinstance(value, list):
        return [render_policy(item, variables) for item in value]
    if isinstance(value, str):
        return Template(value).substitute(variables)
    return value


def main():
    parser = argparse.ArgumentParser(
        description="Create AWS deployment access and configure GitHub repository variables."
    )
    parser.add_argument("--region", default=os.getenv("AWS_REGION", "us-east-1"))
    parser.add_argument("--repo", default=os.getenv("GITHUB_REPOSITORY"))
    parser.add_argument("--state-bucket", default=os.getenv("TF_STATE_BUCKET"))
    parser.add_argument(
        "--state-key", default=os.getenv("TF_STATE_KEY", "tagged/terraform.tfstate")
    )
    parser.add_argument(
        "--role-name", default=os.getenv("AWS_ROLE_NAME", "tagged-deploy")
    )
    parser.add_argument("--oidc-subject", default=os.getenv("GITHUB_OIDC_SUBJECT"))
    args = parser.parse_args()

    repository = args.repo or run(
        "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"
    )
    metadata = json.loads(run("gh", "api", f"repos/{repository}"))
    aws = ["aws", "--region", args.region, "--output", "json"]
    identity = json.loads(run(*aws, "sts", "get-caller-identity"))
    account = identity["Account"]
    partition = identity["Arn"].split(":")[1]
    bucket = args.state_bucket or f"tagged-state-{account}-{args.region}"
    role_arn = f"arn:{partition}:iam::{account}:role/{args.role_name}"
    provider_arn = f"arn:{partition}:iam::{account}:oidc-provider/token.actions.githubusercontent.com"

    owner, name = repository.split("/", 1)
    # GitHub's default OIDC subject includes IDs for newly created repositories.
    if metadata["created_at"][:10] > "2026-07-15":
        owner = f"{owner}@{metadata['owner']['id']}"
        name = f"{name}@{metadata['id']}"
    subject = args.oidc_subject or f"repo:{owner}/{name}:ref:refs/heads/main"
    variables = {
        "partition": partition,
        "region": args.region,
        "account": account,
        "bucket": bucket,
        "state_key": args.state_key,
    }
    policy = render_policy(
        json.loads((ROOT / "script/deployment-policy.json").read_text()), variables
    )

    print(
        f"Bootstrapping {repository} in AWS account {account}, {args.region}.",
        flush=True,
    )
    configure_bucket(aws, bucket, account, args.region)
    configure_provider(aws, provider_arn)
    configure_role(aws, args.role_name, provider_arn, subject, policy)

    repo_variables = {
        "AWS_REGION": args.region,
        "AWS_ROLE_ARN": role_arn,
        "TF_STATE_BUCKET": bucket,
        "TF_STATE_KEY": args.state_key,
    }
    for key, value in repo_variables.items():
        run("gh", "variable", "set", key, "--repo", repository, "--body", value)
        print(f"Set {key}={value}")


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        print(error.stderr or str(error), file=sys.stderr)
        sys.exit(error.returncode)
    except (RuntimeError, KeyError, ValueError) as error:
        print(error, file=sys.stderr)
        sys.exit(1)
