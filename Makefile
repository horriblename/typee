# CC := /bin/gcc

PLATFORM_OUT := platform/zig-out/lib/libroc-gccjit.a

$(PLATFORM_OUT): platform/*
	cd platform && zig build

.PHONY: platform test run-integration
platform: $(PLATFORM_OUT)

app.o: ./*.roc
	# roc build will exit 1 if warnings are present
	roc build --no-link --optimize --output app.o || [ $$? = 1 ]

app: app.o $(PLATFORM_OUT)
	$(CC) -o app -lgccjit $^

.PHONY: run clean

run: app
	./app

clean:
	rm *.o app

integration-test: Debug.roc integration.roc
	roc build integration.roc --output integration-test || true

run-integration: integration-test
	./integration-test

test:
	roc test
