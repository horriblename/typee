const std = @import("std");
const Parser = @import("Parser.zig");
const allocator = std.heap.page_allocator;

test "master test" {
    @import("std").testing.refAllDecls(@This());
}

pub fn main() !void {
    const tokens = try Parser.lexer.parse(allocator, "(+ 10 12)");
    std.debug.print("tokens: {}\n", .{tokens});
}
