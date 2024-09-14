package apis

import "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/notebook/v1"

func init() {
	AddToSchemes = append(AddToSchemes, v1.AddToScheme)
}
