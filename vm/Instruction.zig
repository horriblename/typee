const std = @import("std");

const Memory = @import("Memory.zig");
const Registers = @import("Registers.zig");

const Self = @This();

const Code = packed struct {
    op_code: u4, // TODO: use OpCode
    body: u12,
};

code: Code,

fn fromU16(code: u16) Self {
    return Self{ .code = @bitCast(code) };
}

pub const OpCode = enum(u4) {
    branch = 0,
    add,
    load,
    store,
    jump_to_routine,
    bit_and,
    load_base_offset,
    store_reg,
    rti, // unused
    not,
    load_indirect,
    store_indirect,
    jmp,
    res, // reserved (unused)
    load_effective_addr,
    trap,

    pub fn fromInt(n: u16) ?OpCode {
        if (n > @intFromEnum(OpCode.trap)) {
            return null;
        }
        return @enumFromInt(n);
    }
};

pub const Operation = packed struct {
    op_code: OpCode,

    instruction: packed union {
        branch: BranchInstruction,
        add: AddInstruction,
        load: void,
        store: void,
        jump_to_routine: void,
        bit_and: BitAndInstruction,
        load_base_offset: void,
        store_reg: void,
        rti: void, // unused
        not: void,
        load_indirect: LoadIndirectInstruction,
        store_indirect: void,
        jmp: void,
        res: void, // reserved (unused)
        load_effective_addr: void,
        trap: void,
    },

    pub fn fromU16(code: u16) ?Operation {
        _ = OpCode.fromInt(code) orelse return null;

        // const instruction = switch (op_code) {
        //     .branch => .{ .branch = BranchInstruction },
        //     .add => .{ .add = AddInstruction },
        //     .load => .{ .load = void },
        //     .store => .{ .store = void },
        //     .jump_to_routine => .{ .jump_to_routine = void },
        //     .bit_and => .{ .bit_and = void },
        //     .load_base_offset => .{ .load_base_offset = void },
        //     .store_reg => .{ .store_reg = void },
        //     .rti => .{ .rti = void }, // unused
        //     .not => .{ .not = void },
        //     .load_indirect => .{ .load_indirect = void },
        //     .store_indirect => .{ .store_indirect = void },
        //     .jmp => .{ .jmp = void },
        //     .res => .{ .res = void }, // reserved (unused)
        //     .load_effective_addr => .{ .load_effective_addr = void },
        //     .trap => .{ .trap = void },
        // };

        return @bitCast(code);
    }

    pub fn run(op: Operation, mem: *Memory, reg: *Registers) !void {
        switch (op.op_code) {
            .branch => try op.instruction.branch.run(mem, reg),
            .add => try op.instruction.add.run(mem, reg),
            // .load => {},
            // .store => {},
            // .jump_to_routine => {},
            .bit_and => try op.instruction.bit_and.run(mem, reg),
            // .load_base_offset => {},
            // .store_reg => {},
            // .rti => {}, // unused
            // .not => {},
            .load_indirect => try op.instruction.load_indirect.run(mem, reg),
            // .store_indirect => {},
            // .jmp => {},
            // .res => {}, // reserved (unused)
            // .load_effective_addr => {},
            // .trap => {},
            else => unreachable,
        }
    }
};

const BranchInstruction = packed struct {
    // op_code: u4,
    positive: bool,
    zero: bool,
    negative: bool,
    pc_offset: u9,

    fn run(self: BranchInstruction, mem: *Memory, reg: *Registers) !void {
        _ = mem;
        const pc_offset = sign_extend(u9, self.pc_offset);
        if ((self.negative and reg.negative_flag()) or (self.zero and reg.zero_flag()) or (self.positive and reg.positive_flag())) {
            reg.set(Registers.RegName.pc, reg.get(Registers.RegName.pc) + pc_offset);
        }
    }
};

test "branch" {
    const pc_init = 0x3000;
    const pc_offset = 0x10;
    var mach = setup_machine(&.{}, &.{ .{ .name = Registers.RegName.pc, .val = pc_init }, .{ .name = Registers.RegName.cond, .val = @intFromEnum(Registers.ConditionFlag.pos) } });

    const instruction = Operation{ .op_code = OpCode.branch, .instruction = .{ .branch = BranchInstruction{
        .positive = true,
        .zero = false,
        .negative = false,
        .pc_offset = pc_offset,
    } } };

    try instruction.run(&mach.mem, &mach.reg);

    try std.testing.expectEqual(mach.reg.get(Registers.RegName.pc), pc_init + pc_offset);
}

const AddInstruction = packed struct {
    const ImmediateModeFlag = bool;
    const Source2Spec = union(ImmediateModeFlag) { false: packed struct { filler: u2, src_reg2: u3 }, true: packed struct {
        operand: u5,
    } };

    // op_code: u4,
    dest_reg: u3,
    src_reg1: u3,
    immediate_mode: bool,
    src2: packed union { non_immediate: packed struct { filler: u2, src_reg2: u3 }, immediate: u5 },

    fn run(self: AddInstruction, mem: *Memory, reg: *Registers) !void {
        _ = mem;
        const dest_reg = try Registers.RegName.fromInt(self.dest_reg);
        const src_reg1 = try Registers.RegName.fromInt(self.src_reg1);

        if (self.immediate_mode) {
            reg.set(dest_reg, reg.get(src_reg1) + sign_extend(u5, self.src2.immediate));
        } else {
            const src_reg2 = try Registers.RegName.fromInt(self.src2.non_immediate.src_reg2);
            reg.set(dest_reg, reg.get(src_reg1) + reg.get(src_reg2));
        }
    }
};

test "add: non-immediate mode" {
    var mach = setup_machine(&.{}, &.{ .{ .name = Registers.RegName.r0, .val = 10 }, .{ .name = Registers.RegName.r1, .val = 5 } });

    const instruction = Operation{ .op_code = .add, .instruction = .{
        .add = .{
            .dest_reg = 5,
            .src_reg1 = 0,
            .immediate_mode = false,
            .src2 = .{ .non_immediate = .{
                .filler = 0,
                .src_reg2 = 1,
            } },
        },
    } };

    try instruction.run(&mach.mem, &mach.reg);

    try std.testing.expectEqual(mach.reg.get(Registers.RegName.r5), 15);
}

test "add: immediate mode" {
    var mach = setup_machine(&.{}, &.{.{ .name = Registers.RegName.r0, .val = 10 }});

    const instruction = Operation{
        .op_code = OpCode.add,
        .instruction = .{
            .add = .{
                .dest_reg = @intFromEnum(Registers.RegName.r1),
                .src_reg1 = @intFromEnum(Registers.RegName.r0),
                .immediate_mode = true,
                .src2 = .{ .immediate = 5 },
            },
        },
    };

    try instruction.run(&mach.mem, &mach.reg);

    try std.testing.expectEqual(mach.reg.get(Registers.RegName.r1), 15);
}

const LoadIndirectInstruction = packed struct {
    dest_reg: u3,
    pc_offset: u9,

    fn run(self: LoadIndirectInstruction, mem: *Memory, reg: *Registers) !void {
        const dest_reg = try Registers.RegName.fromInt(self.dest_reg);
        const pc_offset = sign_extend(u9, self.pc_offset);

        reg.set(dest_reg, mem.read(reg.get(Registers.RegName.pc) + pc_offset));
    }
};

test "load_indirect" {
    const pc_init = 0x3000;
    const pc_offset = 0x10;
    var machine = setup_machine(&.{.{ .addr = pc_init + pc_offset, .val = 42 }}, &.{.{ .name = Registers.RegName.pc, .val = pc_init }});

    const instruction = Operation{ .op_code = OpCode.load_indirect, .instruction = .{ .load_indirect = LoadIndirectInstruction{
        .dest_reg = @intFromEnum(Registers.RegName.r0),
        .pc_offset = pc_offset,
    } } };

    try instruction.run(&machine.mem, &machine.reg);
    try std.testing.expectEqual(machine.reg.get(Registers.RegName.r0), 42);
}

const BitAndInstruction = packed struct {
    dest_reg: u3,
    src1_reg: u3,
    immediate_mode: bool,
    src2: Src2,

    const Src2 = packed union {
        immediate: u5,
        non_immediate: packed struct {
            filler: u2,
            src2_reg: u3,
        },
    };

    fn run(self: BitAndInstruction, mem: *Memory, reg: *Registers) !void {
        _ = mem;
        const dest_reg = try Registers.RegName.fromInt(self.dest_reg);
        const src1_reg = try Registers.RegName.fromInt(self.src1_reg);

        if (self.immediate_mode) {
            const operand = sign_extend(u5, self.src2.immediate);
            reg.set(dest_reg, reg.get(src1_reg) & operand);
        } else {
            const src2_reg = try Registers.RegName.fromInt(self.src2.non_immediate.src2_reg);
            reg.set(dest_reg, reg.get(src1_reg) & reg.get(src2_reg));
        }
    }
};

test "bit_and: non-immediate mode" {
    const operand_1 = 0b1010;
    const operand_2 = 0b1100;
    var mach = setup_machine(&.{}, &.{ .{ .name = Registers.RegName.r0, .val = operand_1 }, .{ .name = Registers.RegName.r1, .val = operand_2 } });

    const instruction = Operation{
        .op_code = OpCode.bit_and,
        .instruction = .{
            .bit_and = .{
                .dest_reg = 2,
                .src1_reg = 0,
                .immediate_mode = false,
                .src2 = .{ .non_immediate = .{ .filler = 0, .src2_reg = 1 } },
            },
        },
    };

    try instruction.run(&mach.mem, &mach.reg);

    try std.testing.expectEqual(mach.reg.get(Registers.RegName.r2), operand_1 & operand_2);
}

test "bit_and: immediate mode" {
    const operand_1 = 0b1010;
    const operand_2: u5 = 0b1100;
    var mach = setup_machine(&.{}, &.{ .{ .name = Registers.RegName.r0, .val = operand_1 }, .{ .name = Registers.RegName.r1, .val = operand_2 } });

    const instruction = Operation{ .op_code = OpCode.bit_and, .instruction = .{ .bit_and = .{
        .dest_reg = 2,
        .src1_reg = 0,
        .immediate_mode = true,
        .src2 = .{ .immediate = operand_2 },
    } } };

    try instruction.run(&mach.mem, &mach.reg);

    try std.testing.expectEqual(mach.reg.get(Registers.RegName.r2), operand_1 & operand_2);
}

// TODO: test op_branch

const MemMap = []const struct { addr: u16, val: u16 };
const RegMap = []const struct { name: Registers.RegName, val: u16 };
const Machine = struct { mem: Memory, reg: Registers };
fn setup_machine(mem_init: MemMap, reg_init: RegMap) Machine {
    var machine: Machine = undefined;
    for (mem_init) |pair| {
        machine.mem.write(pair.addr, pair.val);
    }

    for (reg_init) |pair| {
        machine.reg.set(pair.name, pair.val);
    }
    return machine;
}

fn bit_range(bits: u16, comptime from: u4, comptime to: u4) u16 {
    if (from > to) {
        @compileError("bad argument: `from` must be less than `to`");
    }
    const mask = std.math.maxInt(u16) >> (@typeInfo(u16).Int.bits - to - 1);
    return (bits & mask) >> from;
}

fn sign_extend(comptime From: type, x: u16) u16 {
    // FIXME: make type checking work when x: From
    const orig_size = switch (@typeInfo(From)) {
        .Int => |info| if (info.bits <= 16) info.bits else @compileError("int too large"),
        else => @compileError("only ints accepted"),
    };

    // small note: we're using two's complement :c
    const orig_sign_mask = (1 << (orig_size - 1));
    const is_negative = x & orig_sign_mask != 0;

    if (is_negative) {
        // @shlExact seems to be crashing the compiler
        return 1 << 15 | (x ^ orig_sign_mask);
    } else {
        return x;
    }
}

test "sign_extend" {
    std.debug.print("test", .{});
    try std.testing.expectEqual((sign_extend(u5, 0b11111)), 1 << 15 | 0b1111);
    try std.testing.expectEqual((sign_extend(u5, 0b00111)), 0b0111);
}
