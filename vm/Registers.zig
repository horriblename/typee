pub const Error = error{
    BadRegisterNumber,
};

pub const ConditionFlag = enum(u16) {
    pos = 1 << 0,
    zro = 1 << 1,
    neg = 1 << 2,
};

pub const RegName = enum(u4) {
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
    pub fn fromInt(n: u16) !RegName {
        const reg_num: u3 = @intCast(n);
        if (n > @intFromEnum(RegName.count))
            return Error.BadRegisterNumber;

        return @enumFromInt(reg_num);
    }
};

pub const MemoryMappedRegName = enum(u16) {
    kbsr = 0xfe00, // keyboard status
    kbdr = 0xfe02, // keyboard data
};

// pub const Registers = struct {
regs: [RegName.maxValue()]u16,

const Self = @This();

pub fn get(self: *Self, name: RegName) u16 {
    return self.regs[@intFromEnum(name)];
}

pub fn set(self: *Self, name: RegName, value: u16) void {
    self.regs[@intFromEnum(name)] = value;
}

pub fn negative_flag(self: *Self) bool {
    return (self.get(RegName.cond) & @intFromEnum(ConditionFlag.neg)) != 0;
}

pub fn positive_flag(self: *Self) bool {
    return (self.get(RegName.cond) & @intFromEnum(ConditionFlag.pos)) != 0;
}

pub fn zero_flag(self: *Self) bool {
    return (self.get(RegName.cond) & @intFromEnum(ConditionFlag.zro)) != 0;
}

pub fn update_flags(self: *Self, r: RegName) void {
    if (self.get(r) == 0) {
        self.set(RegName.cond, @intFromEnum(ConditionFlag.zro));
    } else if ((self.get(r) >> 15) != 0) { // 1 in the left-most bit indicates negative
        self.set(RegName.cond, @intFromEnum(ConditionFlag.neg));
    } else {
        self.set(RegName.cond, @intFromEnum(ConditionFlag.pos));
    }
}
