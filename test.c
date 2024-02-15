#include <libgccjit.h>
#include <stdio.h>

int main(int argc, char *argv[]) {
  gcc_jit_context *ctx = gcc_jit_context_acquire();
  gcc_jit_type *int_ty = gcc_jit_context_get_type(ctx, GCC_JIT_TYPE_INT);

  gcc_jit_param *param_i = gcc_jit_context_new_param(ctx, NULL, int_ty, "i");
  gcc_jit_function *func = gcc_jit_context_new_function(
      ctx, NULL, GCC_JIT_FUNCTION_EXPORTED, int_ty, "square", 1, &param_i, 0);

  gcc_jit_block *block = gcc_jit_function_new_block(func, NULL);

  gcc_jit_rvalue *expr = gcc_jit_context_new_binary_op(
      ctx, NULL, GCC_JIT_BINARY_OP_MULT, int_ty,
      gcc_jit_param_as_rvalue(param_i), gcc_jit_param_as_rvalue(param_i));

  gcc_jit_block_end_with_return(block, NULL, expr);
  gcc_jit_result *result = gcc_jit_context_compile(ctx);
  gcc_jit_context_release(ctx);

  typedef int (*fn_type)(int);
  fn_type square = gcc_jit_result_get_code(result, "square");
  if (!square) {
    fprintf(stderr, "NULL fn_ptr");
    // goto error;
  }

  printf("result: %d", square(5));
  return 0;
}
