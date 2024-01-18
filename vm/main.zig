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
