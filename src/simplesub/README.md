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
- Bottom: The type of no values - subtype of all types - in some ways, it can be used as a "never"
  type: `int -> Bottom` is a function that never returns

  _insight from [boxbase.org](https://boxbase.org/entries/2020/aug/10/review-of-simple-essence-of-algebraic-subtyping)_
  - the ⊤ is a type that can be constructed from anything because it is discarded and not used at all.
  - The ⊥ is a type that cannot be constructed because it can be used in every way.


- Principal Type: as far as I understand, the principal type property is, given an environment and
  a term, you can always infer a type of the term where all possible types are subtypes of the
  inferred type (aka principal type).

  The importance seems to be in getting "fully inferred" types - i.e. for any valid program type
  annotations are completely unnecessary

Future points of interest:

- [generalization via levels](https://okmij.org/ftp/ML/generalization.html#levels)
