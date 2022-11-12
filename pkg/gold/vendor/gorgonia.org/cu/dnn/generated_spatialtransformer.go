package cudnn

/* Generated by gencudnn. DO NOT EDIT */

// #include <cudnn.h>
import "C"
import (
	"runtime"
)

// SpatialTransformer is a representation of cudnnSpatialTransformerDescriptor_t.
type SpatialTransformer struct {
	internal C.cudnnSpatialTransformerDescriptor_t

	samplerType SamplerType
	dataType    DataType
	nbDims      int
	dimA        []int
}

// NewSpatialTransformer creates a new SpatialTransformer.
func NewSpatialTransformer(samplerType SamplerType, dataType DataType, nbDims int, dimA []int) (retVal *SpatialTransformer, err error) {
	var internal C.cudnnSpatialTransformerDescriptor_t
	if err := result(C.cudnnCreateSpatialTransformerDescriptor(&internal)); err != nil {
		return nil, err
	}

	dimAC, dimACManaged := ints2CIntPtr(dimA)
	defer returnManaged(dimACManaged)
	if err := result(C.cudnnSetSpatialTransformerNdDescriptor(internal, samplerType.C(), dataType.C(), C.int(nbDims), dimAC)); err != nil {
		return nil, err
	}

	retVal = &SpatialTransformer{
		internal:    internal,
		samplerType: samplerType,
		dataType:    dataType,
		nbDims:      nbDims,
		dimA:        dimA,
	}
	runtime.SetFinalizer(retVal, destroySpatialTransformer)
	return retVal, nil
}

// SamplerType returns the internal samplerType.
func (s *SpatialTransformer) SamplerType() SamplerType { return s.samplerType }

// DataType returns the internal dataType.
func (s *SpatialTransformer) DataType() DataType { return s.dataType }

// NbDims returns the internal nbDims.
func (s *SpatialTransformer) NbDims() int { return s.nbDims }

//TODO: "cudnnSetSpatialTransformerNdDescriptor": Parameter 4 Skipped "dimA" of const int[] - unmapped type

func destroySpatialTransformer(obj *SpatialTransformer) {
	C.cudnnDestroySpatialTransformerDescriptor(obj.internal)
}
