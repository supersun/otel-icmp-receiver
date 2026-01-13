
.PHONY: test
test:
	go test -v ./...

.PHONY: update-deps
update-deps:
	go get -u -v ./...
	go mod tidy


#github.com/supersun/otel-icmp-receiver/-/tags
REMOTE?=github.com/supersun/otel-icmp-receiver.git
.PHONY: push-tags
push-tags:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Adding tag ${TAG}"
	@#git tag -a ${TAG} -s -m "Version ${TAG}"
	@git tag -a ${TAG} -m "Version ${TAG}"
	@echo "Pushing tag ${TAG}"
	@git push ${REMOTE} ${TAG}

# Used for debug only
.PHONY: delete-tags
delete-tags:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Deleting local tag ${TAG}"
	@if [ -n "$$(git tag -l ${TAG})" ]; then \
		git tag -d ${TAG}; \
	fi
	@echo "Deleting remote tag ${TAG}"
	@git push ${REMOTE} :refs/tags/${TAG}

# Used for debug only
.PHONY: repeat-tags
repeat-tags: delete-tags push-tags