#include "std.c"
#include <stdio.h>
#include <string.h>

#define TEST(msg, steps)                                                       \
  do {                                                                         \
    int _test_failed = 0;                                                      \
    fprintf(stderr, "=============\nRunning test: %s\n", (msg));               \
    (steps);                                                                   \
    if (_test_failed) {                                                        \
      fprintf(stderr, "\x1b[31m FAIL \x1b[0m%s\n", (msg));                     \
    } else {                                                                   \
      fprintf(stderr, "\x1b[32m OK \x1b[0m\n");                                \
    }                                                                          \
  } while (0);

#define CHECK(expr)                                                            \
  if (!(expr)) {                                                               \
    fprintf(stderr, "line %d: failed assertion: %s\n", __LINE__, #expr);       \
    _test_failed = 1;                                                          \
  }

#define CHECK_EQ(lhs, rhs)                                                     \
  if ((lhs) != (rhs)) {                                                        \
    fprintf(stderr, "line %d: failed assertion: %s == %s\n", __LINE__, #lhs,   \
            #rhs);                                                             \
    fprintf(stderr, "  left:  %ld\n", (lhs));                                  \
    fprintf(stderr, "  right: %ld\n", (rhs));                                  \
    _test_failed = 1;                                                          \
  }

#define CHECK_NEQ(lhs, rhs)                                                    \
  if ((lhs) == (rhs)) {                                                        \
    fprintf(stderr, "line %d: failed assertion: %s != %s\n", __LINE__, #lhs,   \
            #rhs);                                                             \
    fprintf(stderr, "  left:  %ld\n", (lhs));                                  \
    fprintf(stderr, "  right: %ld\n", (rhs));                                  \
    _test_failed = 1;                                                          \
  }

static int saidHi = 0;
void sayHi(void *data) {
  printf("hi %s\n", (char *)data);
  saidHi = 1;
}
typedef void F0(void *data);
void callClosure(void *c, void *data) {
  Closure *closure = c;
  ((F0 *)closure->func)(closure->data);
}

int main() {
  TEST("list append: empty buffer", {
    List l = newList();
    int item = 42;
    CHECK_EQ(l.size, 0);
    CHECK_EQ(l.data, NULL);

    List l2 = listAppend(l, sizeof(item), &item);

    CHECK_EQ(l2.size, 1);
    CHECK(l2.data);
    CHECK_EQ(l2.data->size, 1);
    CHECK_EQ(l2.data->cap, LIST_INITIAL_CAP);
    int get0;
    listGet(l2, sizeof(int), 0, &get0);
    CHECK_EQ(get0, item);
  });

  TEST("list append: append to already appended buffer", {
    List l = newList();
    int item0 = 1;
    int item1 = 2;
    int item2 = 3;
    int result;

    l = listAppend(l, sizeof(item0), &item0);
    List l1 = listAppend(l, sizeof(item1), &item1);
    List l2 = listAppend(l, sizeof(item2), &item2);

    CHECK_EQ(l1.size, 2);
    CHECK_EQ(l1.data->size, 2);
    CHECK_EQ(l1.data->cap, LIST_INITIAL_CAP);

    listGet(l1, sizeof(int), 0, &result);
    CHECK_EQ(result, item0);

    listGet(l1, sizeof(int), 1, &result);
    CHECK_EQ(result, item1);

    CHECK_NEQ(l1.data, l2.data);

    CHECK_EQ(l2.size, 2);
    CHECK_EQ(l2.data->size, 2);
    CHECK_EQ(l2.data->cap, LIST_INITIAL_CAP);

    listGet(l2, sizeof(int), 0, &result);
    CHECK_EQ(result, item0);

    listGet(l2, sizeof(int), 1, &result);
    CHECK_EQ(result, item2);
  });

  TEST("list append: append to undersized buffer", {
    int64_t initial_cap = 1;
    List l = preallocateList(initial_cap, sizeof(int));
    int item0 = 1;
    int item1 = 2;

    List l1 = listAppend(l, sizeof(item0), &item0);
    List l2 = listAppend(l1, sizeof(item1), &item1);

    CHECK_EQ(l2.size, 2);
    CHECK_EQ(l2.data->size, 2);
    CHECK_EQ(l2.data->cap, initial_cap * LIST_GROWTH_RATE);

    int result0;
    int result1;
    listGet(l2, sizeof(int), 0, &result0);
    listGet(l2, sizeof(int), 1, &result1);

    CHECK_EQ(result0, item0);
    CHECK_EQ(result1, item1);
  })

  TEST("list foreach: call closures", {
    List l = newList();
    Closure item0;
    item0.func = sayHi;
    item0.data = "John";
    item0.cleanup = NULL;
    l = listAppend(l, sizeof(Closure), &item0);

    Closure caller = {0};
    caller.func = callClosure;
    CHECK_EQ(saidHi, 0);
    listForEach(l, sizeof(Closure), caller);
    CHECK_EQ(saidHi, 1);
  })

  TEST("i64ToStr", {
    Str s = i64ToStr(12345);
    CHECK_EQ(s.size, 5);
    CHECK_EQ(strncmp(s.data, "12345", s.size), 0);
  })
}
