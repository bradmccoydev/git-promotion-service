package replacer

import (
	"regexp"
	"strings"

	logger "github.com/sirupsen/logrus"
)

const prefix = `{"keptn.git-promotion.replacewith":"`
const suffix = `"}`

// Replace value marked by yaml comment e.g.
// tag: 2.5.5 # {"keptn.git-promotion.replacewith":"data.image.tag"}
func Replace(fileData string, tags map[string]string) (result string) {
	replaced := fileData
	//quick check for faster processing
	for k, v := range tags {
		if strings.Contains(replaced, prefix+k+suffix) {
			replaced = replaceValue(replaced, k, v)
		}
	}
	logger.WithField("func", "Replace").Infof("tags: %v, original: %s, replaced: %s", tags, fileData, replaced)
	//"tags: map[data.data.message: data.data.project:ortelius data.data.result:pass data.data.service:podtatohead data.data.stage:integration-test data.data.status:succeeded data.data.temporaryData.distributor.subscriptionID:71b1a157-2da1-4a97-91d7-8759f4997473 data.gitcommitid:173c84a53f6d0ce87d455de6fff3b15364eb5485 data.id:3c784266-4146-4387-819d-f5a3be6292b6 data.shkeptncontext:b0501292-d94d-4149-a98d-406aca2b9473 data.source:0xc00019e4a0 data.specversion:1.0 data.time:2022-10-11 07:07:35.695714903 +0000 UTC data.type:0xc00019e4b0 id:3c784266-4146-4387-819d-f5a3be6292b6 source:shipyard-controller specversion:1.0], original: apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n, replaced: apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podtato-head-left-arm\nspec:\n  template:\n    spec:\n      containers:\n      - name: podtato-head-left-arm\n        image: ghcr.io/podtato-head/left-arm:0.2.7\n"
	return replaced
}

func replaceValue(file, key, value string) string {
	splitted := strings.Split(file, "\n")
	annotation := prefix + key + suffix
	re := regexp.MustCompile(`(^.+: ).*( # ` + annotation + `$)`)
	for i, s := range splitted {
		if strings.Contains(s, annotation) {
			splitted[i] = re.ReplaceAllString(s, "${1}"+value+"${2}")
		}
	}
	return strings.Join(splitted, "\n")
}
