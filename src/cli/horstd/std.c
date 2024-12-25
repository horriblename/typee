#include <stddef.h>
#include <string.h>
typedef struct Str {
  char *data;
  size_t size;
} Str;

Str strFromCStr(char *data) {
  Str str = {data, strlen(data)};
  return str;
}
