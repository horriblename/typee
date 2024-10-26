based on [LPTK/simpler-sub](https://github.com/LPTK/simpler-sub)

glossary:

- Polarity of type position:
  - Positive positions correspond to the types that a term _outputs_
  - Negative positions correspond to the types that a term takes as _input_
  - e.g. in `(t0 -> t1) -> t2`, 
    - t2 is in positive position (output of main function)
    - (t0 -> t1) is in position position (input of main function)
    - t1 is in **negative** position (returned by argument function and taken as input)
      - t1 is "provided" by callers via the argument function
    - t0 is in **positive** position (it is provided by the main function when calling the argument
      function)

- Top: The type of all values - supertype of all types
- Bottom: The type of no values - subtype of all types
