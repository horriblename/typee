const std = @import("std");

pub const MEMORY_MAX = 1 << 16;

const Self = @This();
const Registers = @import("Registers.zig");

data: [MEMORY_MAX]u16,

pub fn read(self: *const Self, addr: u16) u16 {
    return mem_read(self, addr);
}

fn mem_read(self: *const Self, addr: u16) u16 {
    if (addr == @intFromEnum(Registers.MemoryMappedRegName.kbsr)) {
        // if (check_key()) {...}
    }

    return self.data[addr];
}

pub fn write(self: *Self, addr: u16, val: u16) void {
    self.data[addr] = val;
}

fn mem_write(self: *Self, addr: u16, val: u16) void {
    self.data[addr] = val;
}

fn mem_dump(self: *const Self) void {
    const max_col: u16 = 16;
    for (self.data, 0..) |slot, i| {
        if (i % max_col == 0) {
            std.debug.print("\n0x{x}\t", .{i});
        }

        std.debug.print("{x:0>2} ", .{slot});
        if (i % 8 == 0) {
            std.debug.print(" ", .{});
        }
    }

    std.debug.print("\n", .{});
}
