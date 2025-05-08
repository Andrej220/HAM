``` mermaid

classDiagram
    %% Main Classes
    class Processor {
        <<interface>>
        +Process([]string, NodeType) ([]string, error)
        +Name() string
    }

    class ProcessorChain {
        -processors map[string]Processor
        -allowEmptyResults bool
        +NewProcessorChain() *ProcessorChain
        +Register(p Processor)
        +Process(lines []string, nodeType NodeType, processorNames ...string) ([]string, error)
        -registerDefaults()
    }

    %% Processor Implementations
    class TrimProcessor {
        +Name() string
        +Process([]string, NodeType) ([]string, error)
    }

    class KeyValueProcessor {
        +Name() string
        +Process([]string, NodeType) ([]string, error)
        -parseKeyValueLines(lines []string) (map[string]string, error)
    }

    class KeyValueJSONProcessor {
        +Name() string
        +Process([]string, NodeType) ([]string, error)
    }

    class SplitLinesProcessor {
        +Name() string
        +Process([]string, NodeType) ([]string, error)
    }

    %% Constants (as Enums)
    class NodeType {
        <<enumeration>>
        NodeTypeObject
        NodeTypeString
        NodeTypeArray
    }

    class ProcessorType {
        <<enumeration>>
        ProcessorTypeTrim
        ProcessorTypeKeyValue
        ProcessorJSONTypeKeyValue
        ProcessorTypeSplitLines
    }

    %% Relationships
    Processor <|.. TrimProcessor : implements
    Processor <|.. KeyValueProcessor : implements
    Processor <|.. KeyValueJSONProcessor : implements
    Processor <|.. SplitLinesProcessor : implements

    ProcessorChain "1" *-- "*" Processor : contains
    ProcessorChain ..> NodeType : uses
    ProcessorChain ..> ProcessorType : uses

    KeyValueProcessor ..> NodeType : uses
    SplitLinesProcessor ..> NodeType : uses
    KeyValueJSONProcessor ..> NodeType : uses

```