# PackObjectsHook

When Goblet serves request, it will run `git upload-pack`. When configured with [uploadpack.packObjectsHook](
https://git-scm.com/docs/git-config#Documentation/git-config.txt-uploadpackpackObjectsHook), git will call the hook with all the usual pack-objects arguments. This is our opportunity to cache the result.

Similar act has been implemented by Gitaly (see [design](https://gitlab.com/gitlab-org/gitaly/-/blob/master/doc/design_pack_objects_cache.md) and [code](https://gitlab.com/gitlab-org/gitaly/-/blob/master/internal/gitaly/service/hook/pack_objects.go)). The basic idea is summarised in this [blog post](https://about.gitlab.com/blog/2022/02/07/git-fetch-performance-2021-part-2/): 

> You may want to insert a caching layer around pack-objects; it is the most CPU- and memory-intensive part of serving a fetch, and its output is a pure function of its input, making it an ideal place to consolidate identical requests.

Both command arguments and stdin are considered input to the pack-objects call. Thus we compute `key=sha256sum(args+stdin)`, and cache the stdout, shall `exitcode=0`.

File system is used as lock to ensure only one process can generate cache of the same key (see test cases below).

To avoid caching large stdout, you can optionally pass `POH_STDIN_MAX` to only cache if `size(stdin) <= POH_STDIN_MAX`.

## How to test

### Build
```
go build -o /tmp/packobjectshook main.go
```


### Generate cache
```
echo hello world | POH_CACHE_DIR=/tmp/packobjectscache POH_CI_SOURCE=testing /tmp/packobjectshook cat
```
cache will be generated under `/tmp/packobjectscache/key[:2]/key[2:]`.


### Serve cache
Repeat the above command, and the cache will be served. Check `/tmp/packobjectscache/key[:2]/key[2:]/served` for proof.


### Won't cache if stdin exceeds stdin_max
```
echo looooooooooong string | POH_CACHE_DIR=/tmp/packobjectscache POH_CI_SOURCE=testing POH_STDIN_MAX=10 /tmp/packobjectshook cat
```
no cache will be generated under `/tmp/packobjectscache`.


### Won't cache if exitcode != 0
```
echo hello world | POH_CACHE_DIR=/tmp/packobjectscache POH_CI_SOURCE=testing /tmp/packobjectshook false
```
no cache will be generated under `/tmp/packobjectscache`.


### Won't serve cache if cache is not complete
remove `/tmp/packobjectscache/key[:2]/key[2:]/end` from previous example, then run the command again. 
```
echo hello world | POH_CACHE_DIR=/tmp/packobjectscache POH_CI_SOURCE=testing /tmp/packobjectshook cat
```
Notice no new `served` is appended.


### Test racing
```
echo racing | POH_CACHE_DIR=/tmp/packobjectscache POH_CI_SOURCE=testing /tmp/packobjectshook sleep 5 &
echo racing | POH_CACHE_DIR=/tmp/packobjectscache POH_CI_SOURCE=testing /tmp/packobjectshook sleep 5 &
echo racing | POH_CACHE_DIR=/tmp/packobjectscache POH_CI_SOURCE=testing /tmp/packobjectshook sleep 5 &
```
After 5 seconds, check that there is only 1 line at `/tmp/packobjectscache/key[:2]/key[2:]/end` (if multiple processes can write to the same cache entry, you would have seen multiple lines).

Fun fact, if you run the racing command again, they will return immediately, and `served` will show 3 records.
