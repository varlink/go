all: certification/orgvarlinkcertification/orgvarlinkcertification.go
	go test ./...

certification/orgvarlinkcertification/orgvarlinkcertification.go: certification/orgvarlinkcertification/org.varlink.certification.varlink
	go generate certification/orgvarlinkcertification/generate.go

.PHONY: all
