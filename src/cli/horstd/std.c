#include <stdint.h>
#include <stdio.h>
#include <string.h>
typedef struct Str {
  char *data;
  int64_t size;
} Str;

Str strFromCStr(char *data) {
  Str str = {data, strlen(data)};
  return str;
}

char *strAsCStr(Str s) { return s.data; }

void print(Str s) { fwrite(s.data, 1, s.size, stdout); }
