# CC := /bin/gcc

PLATFORM_OUT := platform/zig-out/lib/libroc-gccjit.a

$(PLATFORM_OUT): platform/*
	cd platform && zig build

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
