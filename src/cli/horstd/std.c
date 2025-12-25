#include <assert.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
typedef struct Str {
  char *data;
  int64_t size;
} Str;

Str strFromCStr(char *data) {
  Str str = {data, strlen(data)};
  return str;
}

// should I use uint8_t instead of char?
char *strToCStr(Str s) {
  char *dest = calloc(sizeof(char), s.size + 1);
  strncpy(dest, s.data, s.size);
  return dest;
}

void print(Str s) { fwrite(s.data, 1, s.size, stdout); }

int32_t i64ToI32(int64_t x) { return x; }
int8_t i64ToI8(int64_t x) { return x; }

/*
 * Closure
 */

typedef struct Closure {
  void *func;
  void *data;
  void *cleanup;
} Closure;

/*
 * Lists
 */

// initial allocation cap if an allocation is required without a specified cap
#define LIST_INITIAL_CAP 8
#define LIST_GROWTH_RATE 2

// PRINT_LIST(list, int, "%d");
#define PRINT_LIST(l, Typ, fmt)                                                \
  do {                                                                         \
    for (int64_t i = 0; i < (l).size; i++) {                                   \
      Typ _val;                                                                \
      listGet(l, sizeof(Typ), i, &_val);                                       \
      printf((fmt), _val);                                                     \
    }                                                                          \
    printf("\n");                                                              \
  } while (0)

// this goes on the heap
typedef struct ListData {
  int64_t ref_count;
  int64_t size;
  int64_t cap;
  char data[];
} ListData;

// this goes on the stack
typedef struct List {
  int64_t size;
  ListData *data;
} List;

List newList() {
  List l = {.size = 0, .data = NULL};
  return l;
}

List preallocateList(int64_t cap, int64_t item_size) {
  // zero-out ListData.data?
  ListData *data = calloc(sizeof(ListData) + cap * item_size, 1);
  data->ref_count = 1;
  data->size = 0;
  data->cap = cap;
  List l = {.size = 0, .data = data};
  return l;
}

// returns bool actually
static int64_t listDataHasSpace(List l) {
  // a list has space for an append if:
  // - the ListData.size < ListData.cap, and
  // - the List.size == ListData.size (meaning no other slice appended to this)
  //   (List.size < ListData.size is impossible right?)
  return l.data && l.data->cap > l.data->size && l.size == l.data->size;
}

// I should figure out an "ABI" for passing generic values
List listAppend(List l, int64_t item_size, void *thing) {
  List newL;
  if (listDataHasSpace(l)) {
    l.data->size++;
    memcpy(&l.data->data[l.size * item_size], thing, item_size);
    newL.size = l.size + 1;
    newL.data = l.data;
    return newL;
  }

  if (!l.data) {
    newL = preallocateList(LIST_INITIAL_CAP, item_size);
    memcpy(&newL.data->data[l.size * item_size], thing, item_size);
    newL.size++;
    newL.data->size++;
    return newL;
  }

  // someone else appended to the buffer, we copy it over but no need to
  // grow buffer size
  if (l.data->size < l.data->cap) {
    newL = preallocateList(l.data->cap, item_size);
  } else {
    // buffer too small, grow it
    newL = preallocateList(l.data->cap * LIST_GROWTH_RATE, item_size);
  }

  memcpy(newL.data->data, l.data->data, l.size * item_size);
  newL.data->size = l.size + 1;
  newL.size = l.size + 1;
  memcpy(&newL.data->data[l.size * item_size], thing, item_size);
  return newL;
}

void listGet(List l, int64_t item_size, int64_t i, void *result) {
  assert(l.data);
  assert(i < l.size && i < l.data->size);
  memcpy(result, l.data->data + i * item_size, item_size);
}

typedef void ListForEachFunc(void *item, void *data);

void listForEach(List l, int64_t item_size, Closure f) {
  for (int64_t i = 0; i < l.size; i++) {
    ((ListForEachFunc *)(f.func))(l.data->data + i * item_size, f.data);
  }
}

int64_t len(List l) { return l.size; }

void *emptyList() { return NULL; }
