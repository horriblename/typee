// use nom::Parser;
// use typee::{lex::lex, parse::parse_tokens, backend::build_to_file};
use std::fs::{File, read_to_string};
use clap::{arg, command};
use typee::{parse::parse_tokens, backend::build_to_file};

fn main() {
    let matches = command!()
        .arg(arg!(<in_file> "Input file name"))
        .arg(arg!([out_file] "Output file name").default_value("a.out"))
        .get_matches();

    let in_path = matches.get_one::<String>("in_file").unwrap();
    let out_path = matches.get_one::<String>("out_file").unwrap();

    let source = read_to_string(&in_path).unwrap();

    let (_, tokens) = typee::lex::lex(&source).unwrap();
    let (_, ast) = parse_tokens(&tokens).unwrap();
    build_to_file(&ast, &out_path);
}

