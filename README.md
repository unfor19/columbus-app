# columbus-app

[![Release Binaries](https://github.com/unfor19/columbus-app/actions/workflows/release.yml/badge.svg)](https://github.com/unfor19/columbus-app/actions/workflows/release.yml)

Discover AWS resources according to a public endpoint. The name Columbus is derived from the need to explore, discover, and be amazed of the things you can find in your AWS account (America).

This is a work-in-progress; The whole idea is to provide **insights** and not raw data. Examples of desired insights, based on past experience of provisioning AWS resources:

1. Check if an S3 bucket is configured properly to be served by a CloudFront distribution (S3 Origin with Static website hosting **disabled**)
1. Check if an S3 bucket is protected behind a CloudFront distribution (S3 Bucket Policy + OAI, S3 responseStatusCode=403, and Cloudfront responseStatusCode=200)
1. Check if the `index.html` that is hosted in S3, matches the one that is served by the CloudFront distribution (Remedy with CloudFront invalidation)
1. The list goes on an on, and it's all based on past experience, and routine checks that in my opinion, should be automatic.

## Requirements

- AWS Account and [User with AdministratorAccess](https://docs.aws.amazon.com/IAM/latest/UserGuide/getting-started_create-admin-group.html)
- A public endpoint that its resources are hosted in AWS, I'm using https://dev.sokker.info for testing purposes. Please note that you **must use your own endpoint**, otherwise the whole process in AWS will fail immediately

## Getting Started

1. Download the application from the [Releases page](https://github.com/unfor19/columbus-app/releases) > Assets > Relevant OS binary
2. Set AWS Credentials
   ```bash
   # AWS Credentials Precedence
   # 1. Environment Variables
   # 2. Config/Profile
   ``` 
3. Run the application
   ```bash
   # Linux - Assuming you downloaded the relevant binary
   chmod +x columbus-app_0.0.1rc4_linux_386
   ./columbus-app_0.0.1rc4_linux_386

   2022/01/28 16:34:19 Starting server ...
   [GIN-debug] [WARNING] Creating an Engine instance with the Logger and Recovery middleware already attached.

   [GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
   - using env:   export GIN_MODE=release
   - using code:  gin.SetMode(gin.ReleaseMode)

   [GIN-debug] GET    /explore                  --> main.main.func1 (3 handlers)
   [GIN-debug] GET    /                         --> main.main.func2 (3 handlers)
   [GIN-debug] Listening and serving HTTP on :8080
   ```
4. Before querying the Columbus server, modify this URL by changing `REQUEST_URL`
   ```bash
   http://localhost:8080/explore?requestUrl=REQUEST_URL
   ```

   For example (won't your in your AWS account)
   ```bash
   http://localhost:8080/explore?requestUrl=https://dev.sokker.info
   ```
5. Columbus explores your AWS account, according to the `REQUEST_URL` and sends back a response with insights (currently a raw JSON object see the below example response)

<details>

<summary>Columbus Response Example - Expand/Collapse</summary>

```json
{
  "CloudFrontOrigins": [
    {
      "OriginType": "s3-bucket",
      "OriginName": "dev.sokker.info",
      "OriginUrl": "dev.sokker.info.s3.eu-west-1.amazonaws.com",
      "OriginPath": "",
      "OriginIndexETag": "078043f7839a926cbb494b984e1c9956",
      "OriginBucketPolicy": {
        "Version": "2012-10-17",
        "Statement": [
          {
            "Sid": "PublicReadForGetBucketObjects",
            "Effect": "Allow",
            "Action": "s3:GetObject",
            "Resource": "arn:aws:s3:::dev.sokker.info/*",
            "Principal": {
              "AWS": "arn:aws:iam::cloudfront:user/CloudFront Origin Access Identity EABC0KIJFBSUUS"
            }
          }
        ],
        "Id": "DeployBucketPolicy"
      },
      "OriginBucketPolicyIsPublic": false,
      "OriginResourceExists": true,
      "OriginIsWebsite": false,
      "OriginUrlResponse": {
        "StatusCode": 403,
        "Headers": [
          { "Name": "Date", "Value": "Fri, 28 Jan 2022 14:27:47 GMT" },
          { "Name": "Server", "Value": "AmazonS3" },
          { "Name": "X-Amz-Bucket-Region", "Value": "eu-west-1" },
          { "Name": "X-Amz-Request-Id", "Value": "NA4ABCNQNSTTXWFZ" },
          {
            "Name": "X-Amz-Id-2",
            "Value": "mea3+XcrrP/lqaRyVgYmnMqaUujm+0rPaa6onBCqtzMpBWRGD7U7D+Jsv/JVKBHoU0Ob2j/2W4U="
          },
          { "Name": "Content-Type", "Value": "application/xml" }
        ]
      }
    }
  ],
  "TargetDomain": {
    "DomainName": "dev.sokker.info",
    "RegisteredName": "sokker.info",
    "TargetIpAddress": "13.225.250.115",
    "TargetService": "CLOUDFRONT",
    "UrlResponse": {
      "StatusCode": 200,
      "Headers": [
        { "Name": "X-Cache", "Value": "Hit from cloudfront" },
        {
          "Name": "X-Amz-Cf-Id",
          "Value": "I9YGrb2sO-eXkMBHPLQu5H2DZ5mnu_0fk2b_a0yFhxKwiqUmGBxgAw=="
        },
        { "Name": "Connection", "Value": "keep-alive" },
        { "Name": "Last-Modified", "Value": "Thu, 24 Sep 2020 07:43:31 GMT" },
        { "Name": "Server", "Value": "AmazonS3" },
        {
          "Name": "Via",
          "Value": "1.1 e210e35eb3b86a214f96a9c0bbf8557e.cloudfront.net (CloudFront)"
        },
        { "Name": "X-Amz-Cf-Pop", "Value": "MRS52-P2" },
        { "Name": "Date", "Value": "Thu, 27 Jan 2022 19:14:53 GMT" },
        { "Name": "Content-Length", "Value": "749" },
        { "Name": "Age", "Value": "69167" },
        { "Name": "Content-Type", "Value": "text/html" },
        {
          "Name": "Cache-Control",
          "Value": "public,must-revalidate,proxy-revalidate,max-age=0"
        },
        { "Name": "Accept-Ranges", "Value": "bytes" },
        { "Name": "Etag", "Value": "\"078043f7839a926cbb494b984e1c9956\"" }
      ]
    },
    "EtagResponse": "078043f7839a926cbb494b984e1c9956",
    "Route53Record": "none",
    "WafId": "none",
    "NsLookup": [
      "dev.sokker.info. IN A 52.85.3.119\n",
      "dev.sokker.info. IN A 52.85.3.44\n",
      "dev.sokker.info. IN A 52.85.3.72\n",
      "dev.sokker.info. IN A 52.85.3.7\n"
    ]
  }
}
```

</details>

## Supported Services

1. AWS Route53
   1. Checks if request URL has an existing Hosted Zone and RecordSet
2. AWS CloudFront
   1. Iterates over CloudFront distributions, and checks if request URL matches to any CloudFront distribution by CNAME and/or Origins
   2. Origins
      1. S3
      2. **TODO**: API Gateway
      3. **TODO**: ALB

### TODO

Services that will be supported in the future

1. AWS S3 - partially handled, as part of the CloudFront implementation
1. AWS API Gateway (APIGW)
1. AWS Application Load Balancer (ALB)
1. AWS Network Load Balancer (NLB)
1. AWS Classic Load Balancer (CLB)


## Distribute

- `master` branch - need to add a pipeline
- `releases` - publishing in GitHub as assets for Windows, Linux, and macOS (darwin), see [0.0.1rc1](https://github.com/unfor19/columbus-app/releases/tag/0.0.1rc1)

## Contributing

1. Download and install [go 1.16+](https://golang.org/doc/install)
1. Add this to your `.bashrc` file and reload your terminal
    ```bash
    # GO
    export PATH="$PATH:/usr/local/go/bin"
    export GOPATH="$(go env GOPATH)"
    ```
1. From now on, your Go packages must sit under "$GOPATH", mine is `/home/meir/go`
1. Create a directory for this project
   ```bash
   # Don't change the TARGET_DIR!
   TARGET_DIR="$GOPATH/src/github.com/unfor19" && \
   mkdir -p "$TARGET_DIR" && \
   cd "$TARGET_DIR"

   # Clone
   git clone git@github.com:unfor19/columbus-app.git
   # Or with HTTPS: https://github.com/unfor19/columbus-app.git
   ```
1. Checkout
   ```bash
   git checkout -b feature/awesome
   ```
2. Open your `TARGET_DIR` with your favorite IDE (VSCode)
3. Open a new terminal in your IDE and download the Go dependencies
   ```bash
   make deps
   ```
4. Change some code ...
5. Run the Go application locally
   ```bash
   # Change the request URL
   export COLUMBUS_REQUEST_URL="https://dev.sokker.info"
   make run
   # application's output ...
   ```
6. Build the Go application locally
   ```bash
   make build
   ```
7. Use the artifact
   ```bash
   ./columbus-app
   # application's output ...
   ```
8. If all goes well, push your changes
   ```bash
   git push --set-upstream origin feature/awesome
   ```

## References

- [AWS Code Samples for Go](https://docs.aws.amazon.com/code-samples/latest/catalog/code-catalog-go.html)


## Authors

Created and maintained by [Meir Gabay](https://github.com/unfor19)

## License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/unfor19/columbus-app/blob/master/LICENSE) file for details
