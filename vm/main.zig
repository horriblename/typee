// LC-3 impl, from https://www.jmeiners.com/lc3-vm/
const std = @import("std");

const Error = error{
    UnknownOpCode,
    BadRegisterNumber,
    SteppingOverProgramSize,
};

const State = @import("State.zig");
const Registers = @import("Registers.zig");
const RegName = Registers.RegName;
const Memory = @import("Memory.zig");
const Instruction = @import("Instruction.zig");
const Operation = Instruction.Operation;

const Terminal = struct {
    original_tio: std.os.termios,

    fn disable_input_buffering() !Terminal {
        const original_tio = try std.os.tcgetattr(std.os.STDIN_FILENO);
        var new_tio = original_tio;
        // FIXME:
        // new_tio.c_lflag &= ~std.os.linux.ICANON & ~std.os.linux.ECHO;

        try std.os.tcsetattr(std.os.STDIN_FILENO, std.os.linux.TCSA.NOW, new_tio);

        return .{ .original_tio = original_tio };
    }

    fn restore_input_buffering(self: Terminal) !void {
        try std.os.tcsetattr(std.os.STDIN_FILENO, std.os.linux.TCSA.NOW, self.original_tio);
    }
};

const allocator = std.heap.page_allocator;
pub fn main() !void {
    const args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    if (args.len < 2) {
        std.log.err("lc3 [image-file1] ...\n", .{});
        std.process.exit(2);
    }

    var state = State.init();

    for (args[2..]) |file_path| {
        state.program_size = read_image_from_path(&state.mem, file_path) catch |err| {
            std.log.err("failed to load image at '{s}': {}", .{ file_path, err });
            std.process.exit(1);
        };
    }

    const terminal = Terminal.disable_input_buffering();
    defer if (terminal) |term| {
        term.restore_input_buffering() catch {};
    } else |_| {};

    // TODO: how to trap SIGINT
    // std.os.signalfd(fd: fd_t, mask: *const sigset_t, flags: u32)

    // since exactly one condition flag should be set at any given time, set the z flag
    state.reg.set(RegName.cond, @intFromEnum(Registers.ConditionFlag.zro));

    // set the PC to starting position
    // the first "instruction" points to the starting position
    const pc_start = state.mem_read(0);
    state.reg.set(RegName.pc, pc_start);

    const running = true;

    while (running) {
        try state.step();
    }

    // TODO: shutdown
}

// fn op_add(state: *State, instruction: Instruction) !void {
//     const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
//     const src_reg = try RegName.fromInt(bit_range(instruction, 6, 8));
//     const immediate_mode_flag = bit_range(instruction, 5, 5) == 1;
//
//     if (immediate_mode_flag) {
//         const operand = sign_extend(u5, instruction & 0x1F);
//         const answer = state.reg.get(src_reg) + operand;
//         state.reg.set(dest_reg, answer);
//     } else {
//         const src_reg_2 = try RegName.fromInt(bit_range(instruction, 0, 2));
//         const answer = state.reg.get(src_reg) + state.reg.get(src_reg_2);
//         state.reg.set(dest_reg, answer);
//     }
//
//     update_flags(&state.reg, dest_reg);
// }
//
// test "add: non-immediate mode" {
//     var state = State.init();
//     const dest_reg = RegName.r5;
//     const src1_reg = RegName.r0;
//     const src2_reg = RegName.r1;
//
//     state.reg.set(src1_reg, 10);
//     state.reg.set(src2_reg, 5);
//
//     const opcode = @as(u16, @intFromEnum(OpCode.add)) << 12;
//     const dest_code = @as(u16, @intFromEnum(dest_reg)) << 9;
//     const src1_code = @as(u16, @intFromEnum(src1_reg)) << 6;
//     const src2_code = @as(u16, @intFromEnum(src2_reg));
//
//     const instruction = opcode | dest_code | src1_code | src2_code;
//     try op_add(&state, instruction);
//
//     try std.testing.expectEqual(state.reg.get(dest_reg), 15);
// }
//
// fn sign_extend(comptime From: type, x: u16) u16 {
//     // FIXME: make type checking work when x: From
//     const orig_size = switch (@typeInfo(From)) {
//         .Int => |info| if (info.bits <= 16) info.bits else @compileError("int too large"),
//         else => @compileError("only ints accepted"),
//     };
//
//     // small note: we're using two's complement :c
//     const orig_sign_mask = (1 << (orig_size - 1));
//     const is_negative = x & orig_sign_mask != 0;
//
//     if (is_negative) {
//         // @shlExact seems to be crashing the compiler
//         return 1 << 15 | (x ^ orig_sign_mask);
//     } else {
//         return x;
//     }
// }
//
// test "sign_extend" {
//     try std.testing.expectEqual((sign_extend(u5, 0b11111)), 1 << 15 | 0b1111);
//     try std.testing.expectEqual((sign_extend(u5, 0b00111)), 0b0111);
// }
//
// fn op_load_indirect(state: *State, instruction: Instruction) !void {
//     const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
//     const pc_offset = sign_extend(u9, bit_range(instruction, 0, 8));
//
//     state.reg.set(dest_reg, state.mem_read(state.mem_read(state.reg.get(RegName.pc) + pc_offset)));
// }
//
// test "op_load_indirect" {
//     // TODO
// }
//
// fn op_bit_and(state: *State, instruction: Instruction) !void {
//     const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
//     const src1_reg = try RegName.fromInt(bit_range(instruction, 6, 8));
//     const immediate_mode_flag = bit_range(instruction, 5, 5) != 0;
//
//     if (immediate_mode_flag) {
//         const operand = sign_extend(u5, bit_range(instruction, 0, 4));
//         state.reg.set(dest_reg, state.reg.get(src1_reg) & operand);
//     } else {
//         const src2_reg = try RegName.fromInt(bit_range(instruction, 0, 2));
//         state.reg.set(dest_reg, state.reg.get(src1_reg) & state.reg.get(src2_reg));
//     }
// }
//
// test "op_bit_and: non-immediate mode" {
//     var state = State.init();
//     const operand_1 = 0b1010;
//     const operand_2 = 0b1100;
//
//     state.reg.set(RegName.r0, operand_1);
//     state.reg.set(RegName.r1, operand_2);
//
//     const opcode = @as(u16, @intFromEnum(OpCode.add)) << 12;
//     const dest_code = @as(u16, @intFromEnum(RegName.r2)) << 9;
//     const src1_code = @as(u16, @intFromEnum(RegName.r0)) << 6;
//     const src2_code = @as(u16, @intFromEnum(RegName.r1));
//     const instruction = opcode | dest_code | src1_code | src2_code;
//
//     try op_bit_and(&state, instruction);
//
//     try std.testing.expectEqual(state.reg.get(RegName.r2), operand_1 & operand_2);
// }
//
// test "op_bit_and: immediate mode" {
//     var state = State.init();
//     const operand_1 = 0b1010;
//     const operand_2: u5 = 0b1100;
//
//     state.reg.set(RegName.r0, operand_1);
//     state.reg.set(RegName.r1, operand_2);
//
//     const opcode = @as(u16, @intFromEnum(OpCode.add)) << 12;
//     const dest_code = @as(u16, @intFromEnum(RegName.r2)) << 9;
//     const src1_code = @as(u16, @intFromEnum(RegName.r0)) << 6;
//     const immediate_flag = 1 << 5;
//     const instruction = opcode | dest_code | src1_code | immediate_flag | operand_2;
//
//     try op_bit_and(&state, instruction);
//
//     try std.testing.expectEqual(state.reg.get(RegName.r2), operand_1 & operand_2);
// }
//
// fn op_branch(state: *State, instruction: Instruction) void {
//     const negative = (1 << 11) & instruction != 0;
//     const zero = (1 << 10) & instruction != 0;
//     const positive = (1 << 9) & instruction != 0;
//     const pc_offset = bit_range(instruction, 0, 8);
//
//     if ((negative and state.reg.negative_flag()) or (zero and state.reg.zero_flag()) or (positive and state.reg.positive_flag())) {
//         state.reg.set(RegName.pc, state.reg.get(RegName.pc) + pc_offset);
//     }
// }
//
// // TODO: test op_branch
//
// fn op_jump(state: *State, instruction: Instruction) !void {
//     const base_reg_index = (bit_range(instruction, 6, 8));
//
//     // return
//     if (base_reg_index == 0b111) {
//         state.reg.set(RegName.pc, state.reg.get(RegName.r7));
//     } else {
//         const base_reg = try RegName.fromInt(base_reg_index);
//         state.reg.set(RegName.pc, state.reg.get(base_reg));
//     }
// }
//
// // TODO: test op_jump
//
// fn op_jump_to_routine(state: *State, instruction: Instruction) !void {
//     const offset_mode = (1 << 11) & instruction != 0;
//
//     state.reg.set(RegName.r7, state.reg.get(RegName.pc));
//     if (offset_mode) {
//         const offset = sign_extend(u11, bit_range(instruction, 0, 10));
//         state.reg.set(RegName.pc, state.reg.get(RegName.pc) + offset);
//     } else {
//         const base_reg = try RegName.fromInt(bit_range(instruction, 6, 8));
//         state.reg.set(RegName.pc, state.reg.get(base_reg));
//     }
// }
//
// test "jump_to_routine: with offset" {
//     const pc_init = 0x3000;
//     const pc_offset = 12;
//     var state = State.init();
//     state.reg.set(RegName.pc, pc_init);
//
//     const instruction = build_instruction(OpCode.jump_to_routine, &[_]InstructionComponent{
//         .{ .payload = 1, .offset = 11 }, // offset flag
//         .{ .payload = pc_offset, .offset = 0 }, // offset
//     });
//
//     try op_jump_to_routine(&state, instruction);
//
//     try std.testing.expectEqual(state.reg.get(RegName.r7), pc_init);
//     try std.testing.expectEqual(state.reg.get(RegName.pc), pc_init + pc_offset);
// }
//
// test "jump_to_routine: from register" {
//     const pc_init = 0x3000;
//     const destination = 0x4000;
//     var state = State.init();
//     state.reg.set(RegName.pc, pc_init);
//     state.reg.set(RegName.r0, destination);
//
//     const instruction = build_instruction(OpCode.jump_to_routine, &[_]InstructionComponent{
//         .{ .payload = @intFromEnum(RegName.r0), .offset = 6 }, // offset
//     });
//
//     try op_jump_to_routine(&state, instruction);
//
//     try std.testing.expectEqual(state.reg.get(RegName.r7), pc_init);
//     try std.testing.expectEqual(state.reg.get(RegName.pc), destination);
// }
//
// fn op_load(state: *State, instruction: Instruction) !void {
//     const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
//     const pc_offset = sign_extend(u9, bit_range(instruction, 0, 8));
//
//     const value = state.mem_read(state.reg.get(RegName.pc) + pc_offset);
//     state.reg.set(dest_reg, value);
//
//     update_flags(&state.reg, dest_reg);
// }
//
// test "load" {
//     const pc_init = 0x3000;
//     const pc_offset = 32;
//     var state = State.init();
//     state.program_size = pc_init;
//     state.reg.set(RegName.pc, pc_init);
//
//     const instruction = build_instruction(OpCode.load, &[_]InstructionComponent{
//         .{ .payload = RegName.r0.toInt(), .offset = 9 },
//         .{ .payload = pc_offset, .offset = 0 },
//     });
//     state.mem_write(pc_init, instruction);
//     state.mem_write(pc_init + pc_offset + 1, 42);
//
//     // try op_load(&state, instruction);
//     try state.step();
//
//     try std.testing.expectEqual(state.reg.get(RegName.r0), 42);
// }
//
// fn op_load_base_offset(state: *State, instruction: Instruction) !void {
//     const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
//     const base_reg = try RegName.fromInt(bit_range(instruction, 6, 8));
//     const offset = sign_extend(u6, bit_range(instruction, 0, 5));
//
//     state.reg.set(dest_reg, state.mem_read(state.reg.get(base_reg) + offset));
// }
//
// // TODO: test load_base_offset
//
// fn op_load_effective_addr(state: *State, instruction: Instruction) !void {
//     const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
//     const pc_offset = sign_extend(u9, bit_range(instruction, 0, 8));
//
//     state.reg.set(dest_reg, state.reg.get(RegName.pc) + pc_offset);
//
//     update_flags(&state.reg, dest_reg);
// }
//
// // TODO: test load_effective_addr
//
// fn op_not(state: *State, instruction: Instruction) !void {
//     const dest_reg = try RegName.fromInt(bit_range(instruction, 9, 11));
//     const src_reg = try RegName.fromInt(bit_range(instruction, 6, 8));
//
//     state.reg.set(dest_reg, if (state.reg.get(src_reg) == 0) 1 else 0);
//     update_flags(&state.reg, dest_reg);
// }
//
// test "op not" {
//     var state = State.init();
//     state.reg.set(RegName.r0, 2);
//
//     const instruction = build_instruction(OpCode.not, &[_]InstructionComponent{
//         .{ .payload = @intFromEnum(RegName.r1), .offset = 9 }, // destination
//         .{ .payload = @intFromEnum(RegName.r0), .offset = 6 }, // source
//     });
//
//     try op_not(&state, instruction);
//
//     try std.testing.expectEqual(state.reg.get(RegName.r1), 0);
// }
//
// const InstructionComponent = struct {
//     payload: u16,
//     offset: u4,
// };
//
// fn build_instruction(comptime opcode: OpCode, codes: []const InstructionComponent) u16 {
//     var instruction = @as(u16, @intFromEnum(opcode)) << 12;
//
//     for (codes) |code| {
//         instruction |= code.payload << code.offset;
//     }
//
//     return instruction;
// }
//
// test "build instruction" {
//     const instructions = [_]Instruction{
//         16,
//         build_instruction(OpCode.load, &[_]InstructionComponent{
//             .{ .payload = @intFromEnum(RegName.r1), .offset = 9 },
//             .{ .payload = @as(u9, 2), .offset = 0 },
//         }),
//         build_instruction(OpCode.add, &[_]InstructionComponent{
//             .{ .payload = @intFromEnum(RegName.r1), .offset = 9 }, // destination reg
//             .{ .payload = @intFromEnum(RegName.r0), .offset = 6 }, // source 1 reg
//             .{ .payload = 1, .offset = 5 }, // immediate mode flag
//             .{ .payload = 5, .offset = 0 }, // immediate mode operand
//         }),
//     };
//
//     const file = try std.fs.cwd().createFile("test.bin", .{ .read = true });
//     defer file.close();
//     _ = try file.write(&[_]u8{ 0, 2 }); // origin
//     _ = try file.write(@as(*const [instructions.len * 2]u8, @ptrCast(&instructions)));
// }
//
// fn update_flags(reg: *Registers, r: RegName) void {
//     if (reg.get(r) == 0) {
//         reg.set(RegName.cond, @intFromEnum(ConditionFlag.zro));
//     } else if ((reg.get(r) >> 15) != 0) { // 1 in the left-most bit indicates negative
//         reg.set(RegName.cond, @intFromEnum(ConditionFlag.neg));
//     } else {
//         reg.set(RegName.cond, @intFromEnum(ConditionFlag.pos));
//     }
// }
//
// // 0-indexed, inclusive on both ends
// fn bit_range(bits: u16, comptime from: u4, comptime to: u4) u16 {
//     if (from > to) {
//         @compileError("bad argument: `from` must be less than `to`");
//     }
//     const mask = std.math.maxInt(u16) >> (@typeInfo(u16).Int.bits - to - 1);
//     return (bits & mask) >> from;
// }
//
// test "bit range" {
//     try std.testing.expectEqual(bit_range(0b110, 0, 1), 0b10);
//     try std.testing.expectEqual(bit_range(0b1100, 1, 2), 0b10);
//     try std.testing.expectEqual(bit_range(0b11110000, 2, 5), 0b1100);
// }

const ImageError = error{ProgramTooLarge} || std.fs.File.ReadError || std.fs.File.OpenError;

fn read_image(memory: *Memory, file: std.fs.File) ImageError!u16 {
    // the first 16 bits specify the origin address (where the program should start)
    // var first_instruction: [2]u8 = undefined;
    // try file.read(&first_instruction);
    // const origin = first_instruction[1] << 8 | first_instruction[0];

    // const max_read = MEMORY_MAX - origin;
    // file.readAll(@as([]u8, memory));
    var read = try file.readAll(@as(*[Memory.MEMORY_MAX * 2]u8, @ptrCast(&memory.data)));

    // swap to little endian (by every 16 bits)
    for (0..read / 2) |i| {
        memory.data[i] = swap16(memory.data[i]);
    }

    if (read / 2 > std.math.maxInt(u16)) {
        return ImageError.ProgramTooLarge;
    }

    return @intCast(read / 2);
}

fn swap16(x: u16) u16 {
    return (x << 8) | (x >> 8);
}

fn read_image_from_path(memory: *Memory, image_path: []const u8) ImageError!u16 {
    var file = try std.fs.cwd().openFile(image_path, .{});
    defer file.close();

    return read_image(memory, file);
}

test "master test" {
    @import("std").testing.refAllDecls(@This());
}
