use std::ptr::NonNull;

mod internal;

pub struct Block {
    ptr: BlockPtr,
    size: BlockSize,
}

pub type BlockPtr = NonNull<u8>;
pub type BlockSize = usize;

#[derive(Debug)]
enum BlockError {
    /// Usually means requested block size, and therefore alignment, wasn't a power of two
    BadRequest,
    /// Insufficient memory, couldn't allocate a block
    OOM,
}

pub fn new(size: BlockSize) -> Result<Block, BlockError> {
    if !size.is_power_of_two() {
        return Err(BlockError::BadRequest);
    }

    Ok(Block {
        ptr: internal::alloc_block(size)?,
        size,
    })
}

impl Block {
    pub fn as_ptr(&self) -> *const u8 {
        self.ptr.as_ptr()
    }
}


#[cfg(test)]
mod tests {
    #[test]
    fn block_alignment() {
        // TODO

        // // the block address bitwise AND the alignment bits (size - 1) should
        // // be a mutually exclusive set of bits
        // let mask = size - 1;
        // assert!((block.ptr.as_ptr() as usize & mask) ^ mask == mask);
    }
}
