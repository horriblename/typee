// LC-3 impl, from https://www.jmeiners.com/lc3-vm/
const std = @import("std");

const Error = error{
    UnknownOpCode,
    BadRegisterNumber,
};

const MEMORY_MAX = 1 << 16;
const Memory = [MEMORY_MAX]u16;
const Instruction = u16;

const RegName = enum(u4) {
    r0 = 0,
    r1,
    r2,
    r3,
    r4,
    r5,
    r6,
    r7,
    pc,
    cond,
    count,

    fn maxValue() u4 {
        return @intFromEnum(RegName.count);
    }

    fn toInt(self: RegName) u4 {
        return @intFromEnum(self);
    }

    // sadly I can't find a way to make sure the input's never too big :c
    fn fromInt(n: u16) !RegName {
        const reg_num: u3 = @intCast(n);
        if (n > @intFromEnum(RegName.count))
            return Error.BadRegisterNumber;

        return @enumFromInt(reg_num);
    }
};

const Registers = struct {
    regs: [RegName.maxValue()]u16,

    fn get(self: *Registers, name: RegName) u16 {
        return self.regs[@intFromEnum(name)];
    }

    fn set(self: *Registers, name: RegName, value: u16) void {
        self.regs[@intFromEnum(name)] = value;
    }

    fn negative_flag(self: *Registers) bool {
        return (self.regs[RegName.cond] & ConditionFlag.neg) != 0;
    }

    fn positive_flag(self: *Registers) bool {
        return (self.regs[RegName.cond] & ConditionFlag.pos) != 0;
    }

    fn zero_flag(self: *Registers) bool {
        return (self.regs[RegName.cond] & ConditionFlag.zro) != 0;
    }
};

const State = struct {
    memory: Memory,
    reg: Registers,

    pub fn init() State {
        return State{
            .memory = undefined,
            .reg = undefined,
        };
    }

    pub fn zeroed() State {
        return std.mem.zeroes(State);
    }
};

// note that LC-3 opcodes are 4-bits long
const OpCode = enum(u4) {
    branch = 0,
    add,
    load,
    store,
    jump_to_routine,
    bit_and,
    load_reg,
    store_reg,
    rti, // unused
    not,
    load_indirect,
    store_indirect,
    jmp,
    res, // reserved (unused)
    load_effective_addr,
    trap,

    fn fromInt(n: u16) ?OpCode {
        if (n > @intFromEnum(OpCode.trap)) {
            return null;
        }
        return @enumFromInt(n);
    }
};

const ConditionFlag = enum(u16) {
    pos = 1 << 0,
    zro = 1 << 1,
    neg = 1 << 2,
};

pub fn main() !void {
    const allocator = std.heap.page_allocator;
    const args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    if (args.len < 2) {
        std.log.err("lc3 [image-file1] ...\n", .{});
        std.process.exit(2);
    }

    for (args[2..]) |arg| {
        read_image(arg) catch |err| {
            std.log.err("failed to load image at '{s}': {s}", arg, err);
            std.process.exit(1);
        };
    }

    // TODO: setup
    var state = State.init();

    // since exactly one condition flag should be set at any given time, set the z flag
    state.reg.set(RegName.cond, @intFromEnum(ConditionFlag.zro));

    // set the PC to starting position
    // 0x3000 is the default
    const pc_start = 0x3000;
    state.reg.set(RegName.pc, pc_start);

    const running = true;

    while (running) {
        // FETCH
        const instruction = mem_read(state.reg.get(RegName.pc));
        state.reg.set(RegName.pc, state.reg.get(RegName.pc));
        const op = OpCode.fromInt(instruction >> 12) orelse return Error.UnknownOpCode;

        switch (op) {
            // 0001 | dest_reg(3-bits) | source_reg(3-bits) | mode_bit | args (5-bits)
            // if mode == 0 (called register mode) => args is: 00 | source2 (3-bits)
            // if mode == 1 (called immediate mode) => args is: constant(5-bits)
            OpCode.add => try op_add(&state, instruction),
            // 1010 | dest_reg(3-bits) | PC_offset(9 bits)
            OpCode.load_indirect => try op_load_indirect(&state, instruction),
            OpCode.bit_and => try op_bit_and(&state, instruction),
            OpCode.jmp => try op_jump(&state, instruction),
            OpCode.jump_to_routine => try op_jump_to_routine(&state, instruction),
            else => unreachable,
        }
    }

    // TODO: shutdown
}

fn op_add(state: *State, instruction: Instruction) !void {
    const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
    const src_reg = try RegName.fromInt(bit_range(instruction, 6, 8));
    const immediate_mode_flag = bit_range(instruction, 5, 5) == 1;

    if (immediate_mode_flag) {
        // const operand = sign_extend(u5, instruction & 0x1F);
        // const answer = state.reg.get(src_reg) + operand;
        // state.reg.set(dest_reg, answer);
    } else {
        const src_reg_2 = try RegName.fromInt(bit_range(instruction, 0, 2));
        const answer = state.reg.get(src_reg) + state.reg.get(src_reg_2);
        state.reg.set(dest_reg, answer);
    }

    update_flags(&state.reg, dest_reg);
}

test "add: non-immediate mode" {
    var state = State.init();
    const dest_reg = RegName.r5;
    const src1_reg = RegName.r0;
    const src2_reg = RegName.r1;

    state.reg.set(src1_reg, 10);
    state.reg.set(src2_reg, 5);

    const opcode = @as(u16, @intFromEnum(OpCode.add)) << 12;
    const dest_code = @as(u16, @intFromEnum(dest_reg)) << 9;
    const src1_code = @as(u16, @intFromEnum(src1_reg)) << 6;
    const src2_code = @as(u16, @intFromEnum(src2_reg));

    const instruction = opcode | dest_code | src1_code | src2_code;
    try op_add(&state, instruction);

    try std.testing.expectEqual(state.reg.get(dest_reg), 15);
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
    try std.testing.expectEqual((sign_extend(u5, 0b11111)), 1 << 15 | 0b1111);
    try std.testing.expectEqual((sign_extend(u5, 0b00111)), 0b0111);
}

fn op_load_indirect(state: *State, instruction: Instruction) !void {
    const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
    const pc_offset = bit_range(instruction, 0, 8);

    state.reg.set(dest_reg, mem_read(mem_read(state.reg.get(RegName.pc) + pc_offset)));
}

test "op_load_indirect" {
    // TODO
}

fn op_bit_and(state: *State, instruction: Instruction) !void {
    const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
    const src1_reg = try RegName.fromInt(bit_range(instruction, 6, 8));
    const immediate_mode_flag = bit_range(instruction, 5, 5) != 0;

    if (immediate_mode_flag) {
        const operand = sign_extend(u5, bit_range(instruction, 0, 4));
        state.reg.set(dest_reg, state.reg.get(src1_reg) & operand);
    } else {
        const src2_reg = try RegName.fromInt(bit_range(instruction, 0, 2));
        state.reg.set(dest_reg, state.reg.get(src1_reg) & state.reg.get(src2_reg));
    }
}

test "op_bit_and: non-immediate mode" {
    var state = State.init();
    const operand_1 = 0b1010;
    const operand_2 = 0b1100;

    state.reg.set(RegName.r0, operand_1);
    state.reg.set(RegName.r1, operand_2);

    const opcode = @as(u16, @intFromEnum(OpCode.add)) << 12;
    const dest_code = @as(u16, @intFromEnum(RegName.r2)) << 9;
    const src1_code = @as(u16, @intFromEnum(RegName.r0)) << 6;
    const src2_code = @as(u16, @intFromEnum(RegName.r1));
    const instruction = opcode | dest_code | src1_code | src2_code;

    try op_bit_and(&state, instruction);

    try std.testing.expectEqual(state.reg.get(RegName.r2), operand_1 & operand_2);
}

test "op_bit_and: immediate mode" {
    var state = State.init();
    const operand_1 = 0b1010;
    const operand_2: u5 = 0b1100;

    state.reg.set(RegName.r0, operand_1);
    state.reg.set(RegName.r1, operand_2);

    const opcode = @as(u16, @intFromEnum(OpCode.add)) << 12;
    const dest_code = @as(u16, @intFromEnum(RegName.r2)) << 9;
    const src1_code = @as(u16, @intFromEnum(RegName.r0)) << 6;
    const immediate_flag = 1 << 5;
    const instruction = opcode | dest_code | src1_code | immediate_flag | operand_2;

    try op_bit_and(&state, instruction);

    try std.testing.expectEqual(state.reg.get(RegName.r2), operand_1 & operand_2);
}

fn op_branch(state: *State, instruction: Instruction) void {
    const negative = (1 << 11) & instruction != 0;
    const zero = (1 << 10) & instruction != 0;
    const positive = (1 << 9) & instruction != 0;
    const pc_offset = bit_range(instruction, 0, 8);

    if ((negative and state.reg.negative_flag()) || (zero and state.reg.zero_flag()) || (positive and state.reg.positive_flag())) {
        state.reg.set(RegName.pc, state.reg.get(RegName.pc) + pc_offset);
    }
}

// TODO: test op_branch

fn op_jump(state: *State, instruction: Instruction) !void {
    const base_reg_index = (bit_range(instruction, 6, 8));

    // return
    if (base_reg_index == 0b111) {
        state.reg.set(RegName.pc, state.reg.get(RegName.r7));
    } else {
        const base_reg = try RegName.fromInt(base_reg_index);
        state.reg.set(RegName.pc, state.reg.get(base_reg));
    }
}

// TODO: test op_jump

fn op_jump_to_routine(state: *State, instruction: Instruction) !void {
    const offset_mode = (1 << 11) & instruction != 0;

    state.reg.set(RegName.r7, state.reg.get(RegName.pc));
    if (offset_mode) {
        const offset = sign_extend(u10, bit_range(instruction, 0, 10));
        state.reg.set(RegName.pc, state.reg.get(RegName.pc) + offset);
    } else {
        const base_reg = try RegName.fromInt(bit_range(instruction, 6, 8));
        state.reg.set(RegName.pc, state.reg.get(base_reg));
    }
}

fn update_flags(reg: *Registers, r: RegName) void {
    if (reg.get(r) == 0) {
        reg.set(RegName.cond, @intFromEnum(ConditionFlag.zro));
    } else if ((reg.get(r) >> 15) != 0) { // 1 in the left-most bit indicates negative
        reg.set(RegName.cond, @intFromEnum(ConditionFlag.neg));
    } else {
        reg.set(RegName.cond, @intFromEnum(ConditionFlag.pos));
    }
}

// 0-indexed, inclusive on both ends
fn bit_range(bits: u16, comptime from: u4, comptime to: u4) u16 {
    if (from > to) {
        @compileError("bad argument: `from` must be less than `to`");
    }
    const mask = std.math.maxInt(u16) >> (@typeInfo(u16).Int.bits - to - 1);
    return (bits & mask) >> from;
}

test "bit range" {
    try std.testing.expectEqual(bit_range(0b110, 0, 1), 0b10);
    try std.testing.expectEqual(bit_range(0b1100, 1, 2), 0b10);
    try std.testing.expectEqual(bit_range(0b11110000, 2, 5), 0b1100);
}

fn mem_read(addr: u16) u16 {
    _ = addr;
    unreachable;
}

fn read_image(file_path: [:0]u8) !void {
    _ = file_path;
    unreachable;
}
