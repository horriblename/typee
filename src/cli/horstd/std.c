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
