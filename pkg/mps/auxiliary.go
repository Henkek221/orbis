package mps

import (
	"errors"
	fmt "fmt"
	"regexp"
)

func (rm *ResourceManifest) URLPrefixFromStyleID(styleID ResourceManifest_StyleConfig_StyleID) (pfx string, err error) {
	for _, v := range rm.StyleConfig {
		if v.StyleId == styleID {
			pfx = v.UrlPrefix_1
			break
		}
	}
	if pfx == "" {
		err = errors.New(fmt.Sprint("no url prefix found for style", styleID))
	}
	return
}

func (rm *ResourceManifest) CacheFileNameFromRegexp(regexp *regexp.Regexp) (fn string, err error) {
	for _, cf := range rm.CacheFile {
		if regexp.MatchString(cf.FileName) {
			fn = cf.FileName
			break
		}
	}
	if fn == "" {
		for _, cf := range rm.CacheFile_2 {
			if regexp.MatchString(cf) {
				fn = cf
				break
			}
		}
	}
	if fn == "" {
		err = errors.New(fmt.Sprint("no cache file for regexp", regexp.String()))
	}
	return
}

// CacheBaseURL returns the cache host from both legacy and current resource
// manifests. Apple moved this value from field 31 into the endpoint table in
// field 92, while older manifests still use CacheBaseUrl directly.
func (rm *ResourceManifest) CacheBaseURL() string {
	if rm.CacheBaseUrl != "" {
		return rm.CacheBaseUrl
	}
	if rm.UrlInfoSet != nil && rm.UrlInfoSet.CacheBase != nil {
		return rm.UrlInfoSet.CacheBase.Url
	}
	return ""
}
