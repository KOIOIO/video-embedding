package handler

import objecthandler "video-service/internal/http/handler/objects"

type objectReader = objecthandler.Reader
type ObjectProxyHandler = objecthandler.Handler

func NewObjectProxyHandler(store objectReader) *ObjectProxyHandler {
	return objecthandler.New(store)
}
