**contenttruck is a work in progress. The readme will be updated soon and this notice will go away. Please do not use in production right now.**

# contenttruck

contenttruck is our service that runs on our cluster of CDN nodes at cdn.webscalesoftware.ltd. It allows us to gracefully handle user limits, handle resizing, and generally deliver a high performance user experience.

## Why not use the AWS SDK?

The AWS SDK is fine, but we want to give user tokens a lot more granular control so we can just have the user upload directly to the CDN. We also wish to validate the content before it is uploaded. S3 makes it difficult for us to do that.

## How do I set this up?

Firstly, you will want to download the latest build of Contenttruck. From here, we want to either make a file at `~/.contenttruck.json` (suggested for development) with the keys `secret_access_key`, `access_key_id`, `region`, `bucket_name`, `endpoint`, and `sudo_key`. In production, you will likely want to use environment variables. You can set the following variables:

- `AWS_SECRET_ACCESS_KEY`: Defines the AWS secret access key.
- `AWS_ACCESS_KEY_ID`: Defines the AWS access key ID.
- `AWS_REGION`: Defines the AWS region.
- `AWS_BUCKET_NAME`: Defines the bucket name.
- `AWS_ENDPOINT`: Defines the AWS endpoint.
- `CONTENTTRUCK_SUDO_KEY`: The key that is used to make other keys.

## How do I use this?

All POST requests happen to `/_contenttruck?type=<type>` where `<type>` should be the type specified below. The body should be `application/json` and is also described below.

###
