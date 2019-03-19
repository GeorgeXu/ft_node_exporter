.PHONY: default test

default: local

PUB_DIR = pub

BIN = profwang_probe
NAME = profwang_probe

FT_BIN = ft_node_exporter
FT_NAME = ft_node_exporter

ENTRY = main.go
VERSION := $(shell git describe --always --tags)

# devops 测试环境
TEST_KODO_HOST = http://kodo-testing.prof.wang
TEST_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/$(NAME)/linux/test
FT_TEST_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/$(FT_NAME)/linux/test
TEST_SSL = 0
TEST_PORT = 80

# devops 预发环境
PREPROD_KODO_HOST = https://preprod-kodo.cloudcare.cn
PREPROD_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/$(NAME)/linux/preprod
FT_PREPROD_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/$(FT_NAME)/linux/preprod
PREPROD_SSL = 1
PREPROD_PORT = 443

# 本地搭建的 kodo 测试(XXX: 自行绑定下这个域名到某个地址)
LOCAL_KODO_HOST = http://kodo-testing.prof.wang:9527
LOCAL_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/${NAME}/linux/local
FT_LOCAL_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/${FT_NAME}/linux/local
LOCAL_SSL = 0
LOCAL_PORT = 80

# 正式环境
RELEASE_KODO_HOST = https://kodo.cloudcare.cn
RELEASE_DOWNLOAD_ADDR = cloudcare-files.oss-cn-hangzhou.aliyuncs.com/${NAME}/linux/release
FT_RELEASE_DOWNLOAD_ADDR = cloudcare-kodo.oss-cn-hangzhou.aliyuncs.com/${FT_NAME}/linux/release
RELEASE_SSL = 1
RELEASE_PORT = 443

all: test release pre local alpha

######################################################################################################################
# 本地测试环境
######################################################################################################################
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
	@cp osqueryd kv.json fileinfo.json build/$(NAME)-linux-amd64
	@tar czf $(PUB_DIR)/local/$(NAME)-$(VERSION).tar.gz -C build .
	tree -Csh $(PUB_DIR)

ft_local:
	@echo "===== $(FT_BIN) local ===="
	@rm -rf $(PUB_DIR)/local
	@mkdir -p build $(PUB_DIR)/local
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@go run make.go -main $(ENTRY) -binary $(FT_BIN) -name $(FT_NAME) -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(LOCAL_KODO_HOST) -download-addr $(FT_LOCAL_DOWNLOAD_ADDR) -ssl $(LOCAL_SSL) -port $(LOCAL_PORT) \
		-release local -pub-dir $(PUB_DIR)
	@strip build/$(FT_NAME)-linux-amd64/$(FT_BIN)
	@cp osqueryd kv.json fileinfo.json build/$(FT_NAME)-linux-amd64
	@tar czf $(PUB_DIR)/local/$(FT_NAME)-$(VERSION).tar.gz -C build .
	tree -Csh $(PUB_DIR)

pub_local:
	@echo "publish local ${BIN} ..."
	@go run make.go -pub -release local -pub-dir $(PUB_DIR) -name $(NAME)

ft_pub_local:
	@echo "publish local ${FT_BIN} ..."
	@go run make.go -pub -release local -pub-dir $(PUB_DIR) -name $(FT_NAME)


######################################################################################################################
# devops 测试环境
######################################################################################################################
test:
	@echo "===== $(BIN) test ===="
	@rm -rf $(PUB_DIR)/test
	@mkdir -p build $(PUB_DIR)/test
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@go run make.go -main $(ENTRY) -binary $(BIN) -name $(NAME) -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(TEST_KODO_HOST) -download-addr $(TEST_DOWNLOAD_ADDR) -ssl $(TEST_SSL) -port $(TEST_PORT) \
		-release test -pub-dir $(PUB_DIR)
	@strip build/$(NAME)-linux-amd64/$(BIN)
	@cp osqueryd kv.json fileinfo.json build/$(NAME)-linux-amd64
	@tar czf $(PUB_DIR)/test/$(NAME)-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

ft_test:
	@echo "===== $(FT_BIN) test ===="
	@rm -rf $(PUB_DIR)/test
	@mkdir -p build $(PUB_DIR)/test
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@go run make.go -main $(ENTRY) -binary $(FT_BIN) -name $(FT_NAME) -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(TEST_KODO_HOST) -download-addr $(FT_TEST_DOWNLOAD_ADDR) -ssl $(TEST_SSL) -port $(TEST_PORT) \
		-release test -pub-dir $(PUB_DIR)
	@strip build/$(FT_NAME)-linux-amd64/$(FT_BIN)
	@cp osqueryd kv.json fileinfo.json build/$(FT_NAME)-linux-amd64
	@tar czf $(PUB_DIR)/test/$(FT_NAME)-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

pub_test:
	@echo "publish test ${BIN} ..."
	@go run make.go -pub -release test -pub-dir $(PUB_DIR) -name $(NAME)

ft_pub_test:
	@echo "publish test ${FT_BIN} ..."
	@go run make.go -pub -release test -pub-dir $(PUB_DIR) -name $(FT_NAME)

######################################################################################################################
# 预发环境
######################################################################################################################
pre:
	@echo "===== ${BIN} preprod ===="
	@rm -rf $(PUB_DIR)/preprod
	@mkdir -p build $(PUB_DIR)/preprod
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@go run make.go -main $(ENTRY) -binary $(BIN) -name $(NAME) -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(PREPROD_KODO_HOST) -download-addr $(PREPROD_DOWNLOAD_ADDR) -ssl $(PREPROD_SSL) -port $(PREPROD_PORT) \
		-release preprod -pub-dir $(PUB_DIR)
	@strip build/$(NAME)-linux-amd64/$(BIN)
	@cp osqueryd kv.json fileinfo.json build/$(NAME)-linux-amd64
	@tar czf $(PUB_DIR)/preprod/$(NAME)-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

ft_pre:
	@echo "===== ${FT_BIN} preprod ===="
	@rm -rf $(PUB_DIR)/preprod
	@mkdir -p build $(PUB_DIR)/preprod
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@go run make.go -main $(ENTRY) -binary $(FT_BIN) -name $(FT_NAME) -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(PREPROD_KODO_HOST) -download-addr $(FT_PREPROD_DOWNLOAD_ADDR) -ssl $(PREPROD_SSL) -port $(PREPROD_PORT) \
		-release preprod -pub-dir $(PUB_DIR)
	@strip build/$(FT_NAME)-linux-amd64/$(FT_BIN)
	@cp osqueryd kv.json fileinfo.json build/$(FT_NAME)-linux-amd64
	@tar czf $(PUB_DIR)/preprod/$(FT_NAME)-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

pub_pre:
	@echo "publish preprod ${BIN} ..."
	@go run make.go -pub -release preprod -pub-dir $(PUB_DIR) -name $(NAME)

ft_pub_pre:
	@echo "publish preprod ${FT_BIN} ..."
	@go run make.go -pub -release preprod -pub-dir $(PUB_DIR) -name $(FT_NAME)


######################################################################################################################
# 生产环境
######################################################################################################################

release:
	@echo "===== $(BIN) release ===="
	@rm -rf $(PUB_DIR)/release
	@mkdir -p build $(PUB_DIR)/release
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@go run make.go -main $(ENTRY) -binary $(BIN) -name $(NAME) -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(RELEASE_KODO_HOST) -download-addr $(RELEASE_DOWNLOAD_ADDR) -ssl $(RELEASE_SSL) -port $(RELEASE_PORT) \
		-release release -pub-dir $(PUB_DIR)
	@strip build/$(NAME)-linux-amd64/$(BIN)
	@cp osqueryd kv.json fileinfo.json build/$(NAME)-linux-amd64
	@tar czf $(PUB_DIR)/release/$(NAME)-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

ft_release:
	@echo "===== $(FT_BIN) release ===="
	@rm -rf $(PUB_DIR)/release
	@mkdir -p build $(PUB_DIR)/release
	@mkdir -p git
	@echo 'package git; const (Sha1 string=""; BuildAt string=""; Version string=""; Golang string="")' > git/git.go
	@go run make.go -main $(ENTRY) -binary $(FT_BIN) -name $(FT_NAME) -build-dir build -archs "linux/amd64" -cgo \
		-kodo-host $(RELEASE_KODO_HOST) -download-addr $(FT_RELEASE_DOWNLOAD_ADDR) -ssl $(RELEASE_SSL) -port $(RELEASE_PORT) \
		-release release -pub-dir $(PUB_DIR)
	@strip build/$(FT_NAME)-linux-amd64/$(FT_BIN)
	@cp osqueryd kv.json fileinfo.json build/$(FT_NAME)-linux-amd64
	@tar czf $(PUB_DIR)/release/$(FT_NAME)-$(VERSION).tar.gz autostart -C build .
	tree -Csh $(PUB_DIR)

pub_release:
	@echo "publish release ${BIN} ..."
	@go run make.go -pub -release release -pub-dir $(PUB_DIR) -name $(NAME)

ft_pub_release:
	@echo "publish release ${FT_BIN} ..."
	@go run make.go -pub -release release -pub-dir $(PUB_DIR) -name $(FT_NAME)

clean:
	rm -rf build/*
	rm -rf $(PUB_DIR)/*
