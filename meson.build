project('testy', 'c')

c = meson.get_compiler('c')
libgccjit = c.find_library('gccjit')

executable('testy', 'test.c',
  dependencies: [libgccjit],
  install: true)
