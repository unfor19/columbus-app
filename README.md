# columbus-app

## Requirements

- AWS Account and [User with AdministratorAccess](https://docs.aws.amazon.com/IAM/latest/UserGuide/getting-started_create-admin-group.html)
- A public endpoint that its resources are hosted in AWS, I'm using https://dev.sokker.info for testing purposes. Please note that you **must** use your own endpoint, otherwise the whole process in AWS will fail immediately

## Quick Start

1. Download the application from the [Releases page](https://github.com/unfor19/columbus-app/releases) > Assets > Relevant OS binary
1. Set AWS Credentials and relevant environment variables
   ```bash
   # AWS Credentials Precedence
   # 1. Environment Variables
   # 2. Config/Profile

   # Change this to your public endpoint URL
   export COLUMBUS_REQUEST_URL="https://dev.sokker.info"
   ``` 
1. Run the application
   ```bash
   # Linux - Assuming you downloaded the relevant binary
   chmod +x columbus-app_0.0.1rc1_linux_386
   ./columbus-app_0.0.1rc1_linux_386
   # application's output ...
   ```

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