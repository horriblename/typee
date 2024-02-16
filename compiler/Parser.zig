const std = @import("std");
const mecha = @import("mecha");
const Lexer = @import("Lexer.zig");
const Token = Lexer.Token;

const allocator = std.heap.page_allocator;

const Expr = union(enum) {
    List: std.ArrayList(Expr),
    Int: i32,
    Symbol: []const u8,
};

const Self = @This();

fn tok_lparen(str: []const u8) Token {
    _ = str;
    return Token{ .LParen = {} };
}

fn tok_rparen(str: []const u8) Token {
    _ = str;
    return Token{ .RParen = {} };
}

fn tok_digits(str: []const u8) Token {
    std.debug.print("len: {}\n", .{str.len});
    const num = std.fmt.parseInt(i32, str, 10) catch std.debug.panic("could not parse int token: {s}", .{str});
    return Token{ .IntLiteral = num };
}

fn tok_symbol(str: []const u8) Token {
    return Token{ .Symbol = str };
}

const lparen = token(mecha.string("(")).map(tok_lparen);
const rparen = token(mecha.string(")")).map(tok_rparen);
const digits = token_(mecha.many(digit, .{ .min = 1 })).map(tok_digits);
const symbol = token_(mecha.many(mecha.ascii.not(mecha.oneOf(.{
    parens,
    whitespace,
})), .{})).map(tok_symbol);
const whitespaces = mecha.many(whitespace, .{});

const digit = mecha.ascii.digit(10);
const parens = mecha.oneOf(.{ mecha.ascii.char('('), mecha.ascii.char(')') });
// const lowercase = mecha.ascii.range('a', 'z');
// const uppercase = mecha.ascii.range('A', 'Z');
// const underscore = mecha.ascii.char('_');
const whitespace = mecha.oneOf(.{
    mecha.ascii.char(0x0020),
    mecha.ascii.char(0x000A),
    mecha.ascii.char(0x000D),
    mecha.ascii.char(0x0009),
});

pub const lexer = mecha.combine(.{ mecha.discard(whitespaces), mecha.many(
    mecha.oneOf(.{ lparen, rparen, digits, symbol }),
    .{},
) });

test "lexer" {
    const tokens = try lexer.parse(allocator, "(+ hi bye)");
    std.debug.print("tokens: {}\n", .{tokens});
    const tester = mecha.many(digit, .{ .min = 1 });
    const temp = try tester.parse(allocator, "12");
    std.debug.print("tokens: {}\n", .{temp});
    // _ = try digits.parse(allocator, "1");
}

fn token_(comptime parser: anytype) @TypeOf(parser) {
    return mecha.combine(.{ parser, mecha.discard(whitespaces) });
}
fn token(comptime parser: anytype) mecha.Parser([]const u8) {
    return mecha.combine(.{ parser, mecha.discard(whitespaces) });
}

pub fn init(source: []u8) Self {
    return Self{ .lexer = Lexer.init(source) };
}

// pub fn parse(self: *Self) std.ArrayList(Expr) {
//     result =
// }

// fn parse_expr(self:)
