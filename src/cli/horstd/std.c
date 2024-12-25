#include <stdint.h>
#include <string.h>
typedef struct Str {
  char *data;
  int64_t size;
} Str;

Str strFromCStr(char *data) {
  Str str = {data, strlen(data)};
  return str;
}
