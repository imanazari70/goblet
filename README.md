# Goblet: Git caching proxy

Goblet is a Git proxy server that caches repositories for read access. Git
clients can configure their repositories to use this as an HTTP proxy server,
and this proxy server serves git-fetch requests if it can be served from the
local cache.

In the Git protocol, the server creates a pack-file dynamically based on the
objects that the clients have. Because of this, caching Git protocol response
is hard as different client needs a different response. Goblet parses the
content of the HTTP POST requests and tells if the request can be served from
the local cache.

This was developed by Google to reduce the automation traffic to googlesource.com. 
Goblet would be useful if you need to run a Git read-only mirroring server to offload
the traffic.

We took the initial implementation from 
[`github.com/google/goblet`](https://github.com/google/goblet) at commit 
`140dd10abcdde487161f1e3c2c6e9b4868f7326c` and added the following modifications:
- Removed all code specific to `googlesource.com`.
- Removed all the GCP-related code and dependencies.
- Remove Bazel. Use pure Go tooling to build and release.

## Usage
1. Build Image
    ```bash
    docker build  -t image_name .
    ```
2. Create and run server container
    ```bash
    docker run  -p 8080:8080 image_name
    ```
3. Configure your `Git` client to use `Goblet` as a read-only proxy:
    ```bash
    git config --global url."http://".insteadOf "https://"
    git config --global http.proxy http://server_ip:8080/
    git clone https://chromium.googlesource.com/v8/v8
    ```
4. Try a `git fetch` command and watch `Goblet`'s outputs to see if it's working 
   as expected.
    ```bash
   git fetch origin master
    ```

## Limitations

Note that Goblet forwards the ls-refs traffic to the upstream server. If the
upstream server is down, Goblet is effectively down. Technically, we can modify
Goblet to serve even if the upstream is down, but the current implementation
doesn't do such thing.
