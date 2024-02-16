const std = @import("std");
const assert = std.debug.assert;
const expect = std.testing.expect;

const Self = @This();

source: []const u8,
cursor: usize,

pub const Token = union(enum) {
    LParen: void,
    RParen: void,
    Symbol: []const u8,
    IntLiteral: i32,
};

pub fn init(source: []const u8) Self {
    return Self{ .source = source, .cursor = 0 };
}

fn peek_char(self: *Self) ?u8 {
    if (self.cursor >= self.source.len) {
        return null;
    } else {
        return self.source[self.char];
    }
}

pub fn next(self: *Self) ?Token {
    self.skip_whitespace();
    assert(self.cursor >= self.source.len or self.source[self.cursor] != ' ');

    if (self.cursor >= self.source.len) {
        return null;
    }

    // const start = self.cursor;

    const token = switch (self.source[self.cursor]) {
        '(' => Token.LParen,
        ')' => Token.RParen,
        else => |c| tok: {
            if (is_digit(c)) {
                const number = next_number(self.source[self.cursor..]);
                self.cursor += number.len - 1;
                assert(number.len > 0);

                const num = std.fmt.parseInt(i32, number, 10) catch {
                    unreachable;
                };

                break :tok Token{ .IntLiteral = num };
            } else {
                // TODO: illegal symbol
                const sym = next_symbol(self.source[self.cursor..]);
                self.cursor += sym.len - 1;
                break :tok Token{ .Symbol = sym };
            }
        },
    };

    self.cursor += 1;

    return token;
}

fn next_number(str: []const u8) []const u8 {
    for (str, 0..) |c, i| {
        if (!is_digit(c)) {
            return str[0..i];
        }
    }

    return str;
}

fn next_symbol(str: []const u8) []const u8 {
    assert(is_alpha(str[0]) or is_underscore(str[0]));

    for (str, 0..) |c, i| {
        if (!(is_alpha(c) or is_underscore(c) or is_digit(c))) {
            return str[0..i];
        }
    }

    return str;
}

fn skip_whitespace(self: *Self) void {
    self.cursor += whitespace_to_skip(self.source[self.cursor..]);
}

fn whitespace_to_skip(str: []const u8) usize {
    for (str, 0..) |char, i| {
        if (char != ' ') {
            return i;
        }
    }

    return 0;
}

fn is_alpha(char: u8) bool {
    return (char >= 'a' and char <= 'z') or (char >= 'A' and char <= 'Z');
}

fn is_underscore(char: u8) bool {
    return char == '_';
}

fn is_digit(char: u8) bool {
    return char >= '0' and char <= '9';
}

fn Enumerate(comptime Iter: type, comptime Out: type) type {
    const Out2 = struct { index: usize, payload: Out };

    return struct {
        iter: Iter,
        index: usize,

        const This = @This();
        fn init(iter: Iter) This {
            return This{ .iter = iter, .index = 0 };
        }

        fn next(self: *This) ?Out2 {
            if (self.iter.next()) |payload| {
                const ret = .{ .index = self.index, .payload = payload };
                self.index += 1;
                return ret;
            } else {
                return null;
            }
        }
    };
}

test "lexer" {
    const source = "(def this  12)";
    var lexer = Self.init(source);
    var iter = Enumerate(Self, Token).init(lexer);

    const expected = [_]Token{
        Token{ .LParen = {} },
        Token{ .Symbol = "def" },
        Token{ .Symbol = "this" },
        Token{ .IntLiteral = 12 },
        Token{ .RParen = {} },
    };

    while (iter.next()) |item| {
        std.debug.print("got: {}\n", .{item.payload});
        try expect(@intFromEnum(item.payload) == @intFromEnum(expected[item.index]));
        // try expect(std.meta.eql(item.payload, expected[item.index]));
    }
}
