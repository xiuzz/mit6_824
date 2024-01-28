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


# GFS AIM

1. big fast
2. global
3. sharding 
4. automic recovery
5. single 
6. internal use
7. big sequential access    

# GFS Master

gfs的设计理念为单主节点，Master节点保留用户输入的文件名，每个文件会被分成若干Chunk块(一块64MB)

因此Master还需要保存从file到Chunk的映射(类似于map< filename, array of chunk ID or chunk handle>),在这里chunk id和 chunk handle指的是同一个东西，论文里面一直用的handle,这个老师一直用的id

当然光有chunk id或者 chunk handle是不够的，我们需要每个chunk id到实际chunk的数据的对应关系。于是有了第二个list 或者是 mapping，这里的数据包括：
1. 每个Chunk存储在哪些服务器上，所以这部分是Chunk服务器的列表
2. 每个Chunk当前的版本号，这里可以看出，master节点必须记住每个chunk对应的版本号。
3. 所有对于Chunk的写操作都必须在主Chunk(Primary Chunk)上顺序处理， 主Chunk是Chunk的多个副本之一。这里可以看出，Master节点必须记住哪个Chunk节点持有主节点。
4. 每个主Chunk只能在特定的租约(lease)时间内担任主Chunk，所以master需要记住primary chunk的lease时间

## master存储备份
这是master主要关心的几个数据，他们存储在内存中，内存的读写速度快，当然当master发生故障重启后，数据必然会消失，因此必须要将master中重要的数据备份到存储中。那么那些是需要备份的呢：
1. file 到 Chunk handle的数组(nv)
2. Chunk存储的具体位置信息，即每个chunker的位置，这个是不用的，一旦master上线，那么将可以与还在工作的chunker建立联系，并更新chunk位置表，所以这个是不用的（v）
3. 版本号，这个取决于gfs是如何工作的。这个教授认为是需要写入存储的，老实说，暂时还没理解为什么要写入存储（nv）
4. 主Chunk的标识，这个很简单，不可能写入存储，primary chunk的持续时间为当前的lease时间，每次master重启后，只需要让底下的chunker重新选出primary chunk和当前的lease就行了（v）

5. 除了上述这些，还有就是log要写入磁盘，日志信息是什么样的呢：

   任何时候，如果文件扩展到达了一个新的64MB，需要新增一个Chunk或者由于指定了新的主Chunk而导致版本号更新了，Master节点需要向磁盘中的Log追加一条记录说，我刚刚向这个文件添加了一个新的Chunk或者我刚刚修改了Chunk的版本号。所以每次有这样的更新，都需要写磁盘。GFS论文并没有讨论这么多细节，但是因为写磁盘的速度是有限的，写磁盘会导致Master节点的更新速度也是有限的，所以要尽可能少的写入数据到磁盘。

## master重启过程
当Master节点故障重启，并重建它的状态，你不会想要从log的最开始重建状态，因为log的最开始可能是几年之前，所以Master节点会在磁盘中创建一些checkpoint点，这可能要花费几秒甚至一分钟。这样Master节点重启时，会从log中的最近一个checkpoint开始恢复，再逐条执行从Checkpoint开始的log，最后恢复自己的状态。(类似于游戏的存档机制)


**显示存储的功能是读和写，因此接下来的内容是介绍gfs的write and read**

# GFS Read

对于一个客户端程序，read请求，意味着它有一个filename 和 它想从这个file中读取的某个位置的偏移量(offset)， 然后将这些请求发送给master节点，然后接受返回值。
1. master节点首先从filename查取当前的file的分块
2. 因为每个chunk都是64MB（24位）, 所以偏移量除以64mb将可以从数组中得到这个位置起始的chunk id。
3. 然后master查到含有这个chunk id的服务器生成列表，将这个列表返回给客户端。

现在客户端可以从这些Chunk服务器中挑选一个来读取数据。GFS论文说，客户端会选择一个网络上最近的服务器（Google的数据中心中，IP地址是连续的，所以可以从IP地址的差异判断网络位置的远近），并将读请求发送到那个服务器。因为客户端每次可能只读取1MB或者64KB数据，所以，客户端可能会连续多次读取同一个Chunk的不同位置。所以，客户端会缓存Chunk和服务器的对应关系，这样，当再次读取相同Chunk数据时，就不用一次次的去向Master请求相同的信息。