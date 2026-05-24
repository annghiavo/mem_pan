PYTHON := python3
SERVICE := moderation-fsrs-service
IMAGE := moderation-fsrs:latest

.PHONY: proto venv install run test bench docker docker-size lint clean

proto:
	$(PYTHON) -m grpc_tools.protoc \
		-I proto \
		--python_out=pb \
		--grpc_python_out=pb \
		proto/moderation_fsrs.proto proto/deck_admin.proto
	touch pb/__init__.py

venv:
	$(PYTHON) -m venv .venv

install:
	.venv/bin/pip install -r requirements-dev.txt \
		--index-url https://download.pytorch.org/whl/cpu \
		--extra-index-url https://pypi.org/simple

run:
	$(PYTHON) -m app.main

test:
	$(PYTHON) -m pytest tests/ -v

bench:
	$(PYTHON) tests/benchmarks/bench_latency.py

docker:
	docker build -t $(IMAGE) .

docker-size:
	docker images $(IMAGE) --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"

lint:
	ruff check app/ tests/

clean:
	rm -rf .venv pb/*_pb2*.py
	find . -type d -name __pycache__ -exec rm -rf {} +
