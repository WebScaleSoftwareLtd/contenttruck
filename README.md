# contenttruck

contenttruck is our service that runs on our cluster of CDN nodes at cdn.webscalesoftware.ltd. It allows us to gracefully handle user limits, handle resizing, and generally deliver a high performance user experience.

## Why not use the AWS SDK?

The AWS SDK is fine, but we want to give user tokens a lot more granular control so we can just have the user upload directly to the CDN. We also wish to validate the content before it is uploaded. S3 makes it difficult for us to do that.

## How do I set this up?

To launch the app, you will need to setup your configuration. This can be done in one of two ways:
- Create a `~/.contenttruck.json`: You can set the following properties:
    - "secret_access_key": This is your AWS secret access key.
    - "access_key_id": This is your AWS access key ID.
    - "region": This is the AWS region you want to use.
    - "bucket_name": This is the name of the S3 bucket you want to use.
    - "endpoint": This is the endpoint for your S3-compatible storage provider.
    - "sudo_key": This is a key that grants you superuser access to your contenttruck instance.
    - "http_host": This is the host and port that your contenttruck instance will listen on.
    - "postgres_connection_string": This is the connection string for your Postgres database.
- Set the following environment variables. Note this overrides the JSON config:
    - "AWS_SECRET_ACCESS_KEY": This is your AWS secret access key.
    - "AWS_ACCESS_KEY_ID": This is your AWS access key ID.
    - "AWS_REGION": This is the AWS region you want to use.
    - "AWS_BUCKET_NAME": This is the name of the S3 bucket you want to use.
    - "AWS_ENDPOINT": This is the endpoint for your S3-compatible storage provider.
    - "CONTENTTRUCK_SUDO_KEY": This is a key that grants you superuser access to your contenttruck instance.
    - "HOST": This is the host and port that your contenttruck instance will listen on.
    - "POSTGRES_CONNECTION_STRING": This is the connection string for your Postgres database.

Now simply build, install, or run the container for the app. You might be wondering from here how you interact with this application?

Contenttruck is intended to be interaacted with using SDK's. You probably don't want to write your own, but if you do, the way you interact with the API is:
- Make a POST request to `/_contenttruck` with `X-Type` set to the name of a public function hooked to `apiServer` in `httpserver/api_handlers.go`.
- The request object is the second argument to the function, and the response object is the first output parameter. Note that if there is only 1 output parameter, it can only error or return a 204.
- Most request types require the body to be `Content-Type: application/json`, but for `Upload` specifically, since the body is consumed, you can use `X-Json-Body` to pass the JSON body as a string.
