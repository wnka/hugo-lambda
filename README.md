# Huh?

This Go code is meant to be run in AWS Lambda and is what I use to update [my blog](https://pdp.dev), the source of which is stored on [GitHub](https://github.com/wnka/pdp80-blog) and hosted in S3 and CloudFront.

# How

1. Download the 64-bit Linux version of `hugo` from here: https://github.com/gohugoio/hugo/releases
1. Put the `hugo` binary in the same directory as this repository
1. Create the AWS Lambda bundle: `GOOS=linux  go build main.go && zip main.zip main hugo`
1. This will create `main.zip` which is your Lambda function.

When you create you Lambda function, pick a decent function timeout and memory usage. I use a **1 Minute timeout** and **512MB function memory** and it works for me. YYMV.

Environment variables to set

| Env Var           | Description                                                 | example                                |
|-------------------|-------------------------------------------------------------|----------------------------------------|
| GIT_REPO          | The URL for the Git repository that hosts your Hugo blog    | https://github.com/wnka/pdp80-blog.git |
| S3_BUCKET         | The S3 bucket that hosts your blog                          | pdp80.com                              |
| S3_REGION         | The AWS region your S3 bucket lives in                      | us-east-1                              |
