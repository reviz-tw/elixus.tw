.PHONY: dev serve edit build clean new help

HUGO ?= hugo
EDITOR_DIR := tools/editor

help: ## 顯示可用指令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

dev: ## 同時啟動 Hugo 預覽 (含草稿) 與本地編輯器
	@echo "Hugo  → http://localhost:1313"
	@echo "編輯器 → http://localhost:1314"
	@trap 'kill 0' INT TERM; \
	$(HUGO) server -D --bind 127.0.0.1 --port 1313 & \
	(cd $(EDITOR_DIR) && go run .) & \
	wait

serve: ## 只啟動 Hugo 預覽 (含草稿)
	$(HUGO) server -D --bind 127.0.0.1 --port 1313

edit: ## 只啟動本地 markdown 編輯器 (127.0.0.1:1314)
	cd $(EDITOR_DIR) && go run .

build: ## 產生正式靜態網站到 public/ (不含草稿)
	$(HUGO) --minify --gc

new: ## 用 archetype 建立新文章：make new SLUG=my-post
	@test -n "$(SLUG)" || (echo "用法: make new SLUG=my-post"; exit 1)
	$(HUGO) new posts/$(SLUG).md

clean: ## 清除產生的檔案
	rm -rf public resources/_gen .hugo_build.lock
