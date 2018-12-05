.PHONY: default

default: local

# devops 测试环境
TEST_KODO_HOST = testing.kodo.cloudcare.cn
TEST_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/testing
TEST_SSL = 0
TEST_PORT = 80

# devops 预发环境
PREPROD_KODO_HOST = preprod-kodo.cloudcare.cn
PREPROD_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/preprod
PREPROD_SSL = 1
PREPROD_PORT = 443

# alpha 环境
ALPHA_KODO_HOST = kodo-alpha.cloudcare.cn
ALPHA_DOWNLOAD_ADDR = kodo-agent-alpha.oss-cn-hangzhou.aliyuncs.com/alpha
ALPHA_SSL = 1
ALPHA_PORT = 443

# 本地搭建的 kodo 测试(XXX: 自行绑定下这个域名到某个地址)
LOCAL_KODO_HOST = kodo-local.cloudcare.cn
LOCAL_DOWNLOAD_ADDR = kodo-agent-local-30709.oss-cn-hangzhou.aliyuncs.com/local
LOCAL_SSL = 0
LOCAL_PORT = 80

# 正式环境
KODO_HOST = kodo.cloudcare.cn
DOWNLOAD_ADDR = diaobaoyun-agent.oss-cn-hangzhou.aliyuncs.com
SSL = 1
PORT = 443

PUB_DIR = pub
BIN = corsair
NAME = corsair
ENTRY = node_exporter.go

VERSION := $(shell git describe --always --tags)

all: test release preprod local alpha


local:
	@echo "===== $(BIN) local ===="
	@rm -rf $(PUB_DIR)/local
	@mkdir -p build $(PUB_DIR)/local
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@go run make.go -main $(ENTRY) -binary $(BIN) -name $(NAME) -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(LOCAL_KODO_HOST) -download-addr $(LOCAL_DOWNLOAD_ADDR) -ssl $(LOCAL_SSL) -port $(LOCAL_PORT) \
		-release local -pub-dir $(PUB_DIR)
	@strip build/$(NAME)-linux-amd64/$(BIN)
	@tar czf $(PUB_DIR)/local/$(NAME)-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

alpha:
	@echo "===== agent alpha ===="
	@rm -rf $(PUB_DIR)/alpha
	@mkdir -p build $(PUB_DIR)/alpha
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@cp agent/kodo-widget.min.test.js agent/webtty/bindata/static/js/kodo-widget.min.js
	@cd agent/webtty && go-bindata -prefix bindata -pkg webtty -o resource.go bindata/... && cd ..
	@go run make.go -main main.go -binary agent -name agent -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(ALPHA_KODO_HOST) -download-addr $(ALPHA_DOWNLOAD_ADDR) -ssl $(ALPHA_SSL) \
		-port $(ALPHA_PORT) -release alpha -pub-dir $(PUB_DIR)
	@strip build/agent-linux-amd64/agent
	@tar czf $(PUB_DIR)/alpha/kodo-agent-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

release:
	@echo "===== agent release ===="
	@rm -rf $(PUB_DIR)/release
	@mkdir -p build $(PUB_DIR)/release
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@cp agent/kodo-widget.min.release.js  agent/webtty/bindata/static/js/kodo-widget.min.js
	@cd agent/webtty && go-bindata -prefix bindata -pkg webtty -o resource.go bindata/... && cd ..
	@go run make.go -main main.go -binary agent -name agent -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(KODO_HOST) -download-addr $(DOWNLOAD_ADDR) -ssl $(SSL) -port $(PORT) \
		-release release -pub-dir $(PUB_DIR)
	@strip build/agent-linux-amd64/agent
	@tar czf $(PUB_DIR)/release/kodo-agent-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

test:
	@echo "===== agent test ===="
	@rm -rf $(PUB_DIR)/test
	@mkdir -p build $(PUB_DIR)/test
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@cp agent/kodo-widget.min.test.js agent/webtty/bindata/static/js/kodo-widget.min.js
	@cd agent/webtty && go-bindata -prefix bindata -pkg webtty -o resource.go bindata/... && cd ..
	@go run make.go -main main.go -binary agent -name agent -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(TEST_KODO_HOST) -download-addr $(TEST_DOWNLOAD_ADDR) -ssl $(TEST_SSL) -port $(TEST_PORT) \
		-release test -pub-dir $(PUB_DIR)
	@strip build/agent-linux-amd64/agent
	@tar czf $(PUB_DIR)/test/kodo-agent-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

preprod:
	@echo "===== agent preprod ===="
	@rm -rf $(PUB_DIR)/preprod
	@mkdir -p build $(PUB_DIR)/preprod
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@cp agent/kodo-widget.min.test.js agent/webtty/bindata/static/js/kodo-widget.min.js
	@cd agent/webtty && go-bindata -prefix bindata -pkg webtty -o resource.go bindata/... && cd ..
	@go run make.go -main main.go -binary agent -name agent -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(PREPROD_KODO_HOST) -download-addr $(PREPROD_DOWNLOAD_ADDR) -ssl $(PREPROD_SSL) \
		-port $(PREPROD_PORT) -release preprod -pub-dir $(PUB_DIR)
	@strip build/agent-linux-amd64/agent
	@tar czf $(PUB_DIR)/preprod/kodo-agent-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

pub_local:
	@echo "publish local agent ..."
	@go run make.go -release-agent local -pub-dir $(PUB_DIR)

pub_alpha:
	@echo "publish local agent ..."
	@go run make.go -release-agent alpha -pub-dir $(PUB_DIR)

pub_test:
	@echo "publish test agent ..."
	@go run make.go -release-agent test -pub-dir $(PUB_DIR)

pub_preprod:
	@echo "publish preprod agent ..."
	@go run make.go -release-agent preprod -pub-dir $(PUB_DIR)

pub_release:
	@echo "publish release agent ..."
	@go run make.go -release-agent release -pub-dir $(PUB_DIR)

clean:
	rm -rf build/*
	rm -rf $(PUB_DIR)/*
