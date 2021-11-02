# lambda-bundler

A Lambda function that clones a specific commit from a GitHub repository. It executes language-specific build steps (e.g. `npm install` for Node.js), bundles all the files into a `.zip` archive, and uploads it to S3. This provides a standard build environment for your organization's Lambda function code.

## Depends On

- [artifacts-bucket](../artifacts-bucket) in order to store bundled code.

## How it works

What the bundler does depends on the runtime environment that is being targetted. Currently, two runtimes are supported.

### go1.x

You specify a build command that generates your Lambda function's executable file. The output file should be written to a `dist` folder in the repository's root directory. The lambda-bundler runs your build command and then creates a zip archive of the `dist` folder.

An example command (and the default) would demonstrate this for a repository where the Lambda function's entrypoint is a `main.go` file at the root of the repository:

```
go build -o dist/handler
```

### nodejs14.x

You specify a build command that pulls your application's dependencies and performs any other custom build steps. After running this command, the entire repository's contents (including the `node_modules` folder) are accumulated into a zip archive.

This is the default build command, which simply collects dependencies from npm, while respecting `package-lock.json`:

```
npm ci
```

#### Include paths

If you specify include paths, the zip archive will contain only those paths you specify, plus the `node_modules` folder (if any).

#### Exclude paths

If you specify exclude paths, the zip archive will not contain those paths you specify.
