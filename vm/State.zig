const std = @import("std");

const Registers = @import("Registers.zig");
const Memory = @import("Memory.zig");
const instruction = @import("instruction.zig");
const Operation = instruction.Operation;

pub const Error = error{
    UnknownOpCode,
    BadRegisterNumber,
    SteppingOverProgramSize,
};

const TrapCode = enum(u16) {
    getc = 0x20, // get character from keyboard, not echoed onto terminal
    out = 0x21, // output a character
    puts = 0x22, // output a word string
    in = 0x23, // get character from keyboard, echoed onto terminal
    putsp = 0x24, // output a byte string
    halt = 0x25, // halt the program
};

const Self = @This();

mem: Memory,
reg: Registers,
program_size: u16,

pub fn init() Self {
    var state = Self{
        .mem = undefined,
        .reg = undefined,
        .program_size = undefined,
    };

    // since exactly one condition flag should be set at any given time, set the z flag
    state.reg.set(Registers.RegName.cond, @intFromEnum(Registers.ConditionFlag.zro));

    return state;
}

pub fn mem_read(self: *const Self, addr: u16) u16 {
    if (addr == @intFromEnum(Registers.MemoryMappedRegName.kbsr)) {
        // if (check_key()) {...}
    }

    return self.mem.read(addr);
}

pub fn mem_write(self: *Self, addr: u16, val: u16) void {
    self.mem.write(addr, val);
}

pub fn mem_dump(self: *Self) void {
    const max_col: u16 = 16;
    for (self.mem, 0..) |slot, i| {
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

pub fn step(self: *Self) !void {
    // FETCH
    if (self.reg.get(Registers.RegName.pc) > self.program_size) {
        std.debug.print("stepping over program size; current PC: {}, program size: {}\n", .{ self.reg.get(Registers.RegName.pc), self.program_size });
        return Error.SteppingOverProgramSize;
    }

    const op = Operation.fromU16(self.mem.read(self.reg.get(Registers.RegName.pc))) orelse return Error.UnknownOpCode;
    std.debug.print("PC = 0x{x}; OpCode = {}\n", .{ self.reg.get(Registers.RegName.pc), op });
    self.reg.increment_by(Registers.RegName.pc, 1);

    try op.run(&self.mem, &self.reg);
}

pub fn loop(self: *Self) !void {
    while (true) {
        if (self.step()) {} else |err| {
            if (err == instruction.TrapError.Halt) {
                return;
            }

            return err;
        }
    }
}
