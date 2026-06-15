package handler

import objecthandler "nlp-video-analysis/internal/http/handler/objects"

type objectReader = objecthandler.Reader
type ObjectProxyHandler = objecthandler.Handler

func NewObjectProxyHandler(store objectReader) *ObjectProxyHandler {
	return objecthandler.New(store)
}
