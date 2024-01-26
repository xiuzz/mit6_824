# 课前预习
gfs论文地址：
http://nil.csail.mit.edu/6.824/2020/papers/gfs.pdf

如果说mapreduce是在分析典型的分布式计算系统，那么本章就是在分析一个典型的分布式存储系统

![Alt text](picture/a%20model%20of%20%20gfs.png)

文件被划分为若干个固定大小的chunk（块）。每个chunk被一个不可变的全局唯一的64位chunk handle（块标识符）唯一标识，chunk handle在chunk被创建时由主节点分配。chunkserver将chunk作为Linux文件存储到本地磁盘中，通过chunk handle和byte range（字节范围）来确定需要被读写的chunk和chunk中的数据。为了可靠性考虑，每个chunk会在多个chunkserver中有副本。我们默认存储三份副本，用户也可以为不同的命名空间的域指定不同的副本级别。

master维护系统所有的元数据。元数据包括命名空间（namespace）、访问控制（access control）信息、文件到chunk的映射和chunk当前的位置。master还控制系统级活动如chunk租约（chunk lease）管理、孤儿chunk垃圾回收（garbage collection of orphaned chunks）和chunkserver间的chunk迁移（migration）。master周期性地通过心跳（HeartBeat）消息与每个chunkserver通信，向其下达指令并采集其状态信息。

被链接到应用程序中的GFS client的代码实现了文件系统API并与master和chunkserver通信，代表应用程序来读写数据。进行元数据操作时，client与master交互。而所有的数据（译注：这里指存储的数据，不包括元数据）交互直接由client与chunkserver间进行。因为GFS不提供POXIS API，因此不会陷入到Linux vnode层。

无论client还是chunkserver都不需要缓存文件数据。在client中，因为大部分应用程序需要流式地处理大文件或者数据集过大以至于无法缓存，所以缓存几乎无用武之地。不使用缓存就消除了缓存一致性问题，简化了client和整个系统。（当然，client需要缓存元数据。）chunkserver中的chunk被作为本地文件存储，Linux系统已经在内存中对经常访问的数据在缓冲区缓存，因此也不需要额外地缓存文件数据。
# BIG STORAGE

## WHY HARD

## PERFORMANCE -> SHARDING

## FAULTS -> TOLERANCE

## TOLERACNE -> REPLICATION

## CONSISITENCY -> LOW PEFORMANCE
