project('testy', 'c',
  version: '0.1.0',
  default_options: ['buildtype=release'],
)

c = meson.get_compiler('c')
gccjit = c.find_library('gccjit')

executable('testy', 'test.c', dependencies: [gccjit], install: true)
