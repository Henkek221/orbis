package mps

import (
	"errors"
	fmt "fmt"
	"regexp"

	"google.golang.org/protobuf/encoding/protowire"
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

// StyleVersion returns the URL prefix and the newest announced data version of
// a tile style. Styles newer than this package's enum (for example R3D, 98)
// keep their numeric ID; the version list (field 5) is read from the fields
// the generated code does not model.
func (rm *ResourceManifest) StyleVersion(styleID int32) (prefix string, version int, err error) {
	for _, v := range rm.StyleConfig {
		if int32(v.StyleId) != styleID {
			continue
		}
		prefix = v.UrlPrefix_1
		raw := v.ProtoReflect().GetUnknown()
		for len(raw) > 0 {
			num, typ, n := protowire.ConsumeTag(raw)
			if n < 0 {
				break
			}
			raw = raw[n:]
			m := protowire.ConsumeFieldValue(num, typ, raw)
			if m < 0 {
				break
			}
			if num == 5 && typ == protowire.BytesType {
				inner, _ := protowire.ConsumeBytes(raw)
				for len(inner) > 0 {
					inum, ityp, k := protowire.ConsumeTag(inner)
					if k < 0 {
						break
					}
					inner = inner[k:]
					if inum == 1 && ityp == protowire.VarintType {
						val, _ := protowire.ConsumeVarint(inner)
						if int(val) > version {
							version = int(val)
						}
					}
					l := protowire.ConsumeFieldValue(inum, ityp, inner)
					if l < 0 {
						break
					}
					inner = inner[l:]
				}
			}
			raw = raw[m:]
		}
		break
	}
	if prefix == "" {
		err = fmt.Errorf("manifest has no style %d; the manifest URL may identify an old OS", styleID)
	}
	return
}
