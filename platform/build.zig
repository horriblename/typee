const std = @import("std");

pub fn build(b: *std.Build) !void {
    const target = b.standardTargetOptions(.{});
    const mode = b.standardOptimizeOption(.{});

    const host_lib = b.addStaticLibrary(.{
        .name = "roc-gccjit",
        .root_source_file = .{ .path = "host.zig" },
        .target = target,
        .optimize = mode,
        .link_libc = true,
    });

    host_lib.force_pic = true;
    host_lib.disable_stack_probing = true;

    // FIXME: gccjit broken in nix, use system libs
    host_lib.linkSystemLibrary("gccjit");

    const unit_tests = b.addTest(.{
        .root_source_file = .{ .path = "host.zig" },
        .target = target,
        .optimize = mode,
    });

    const run_unit_tests = b.addRunArtifact(unit_tests);

    const test_step = b.step("test", "Run unit tests");
    test_step.dependOn(&run_unit_tests.step);

    b.installArtifact(host_lib);
}
