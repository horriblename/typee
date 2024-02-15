use crate::eval::gc::{BlockError, BlockPtr, BlockSize};
use std::{ptr::NonNull, alloc::{Layout, alloc, dealloc}};

pub fn alloc_block(size: BlockSize) -> Result<BlockPtr, BlockError> {
    unsafe {
        let layout = Layout::from_size_align_unchecked(size, size);

        let ptr = alloc(layout);
        if ptr.is_null() {
            Err(BlockError::OOM)
        } else {
            Ok(NonNull::new_unchecked(ptr))
        }
    }
}

pub fn dealloc_block(ptr: BlockPtr, size: BlockSize) {
    unsafe {
        let layout = Layout::from_size_align_unchecked(size, size);

        dealloc(ptr.as_ptr(), layout);
    }
}
