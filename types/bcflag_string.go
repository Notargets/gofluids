// Code generated by "stringer -type=BCFLAG"; DO NOT EDIT.

package types

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[BC_None-0]
	_ = x[BC_In-1]
	_ = x[BC_Dirichlet-2]
	_ = x[BC_Slip-3]
	_ = x[BC_Far-4]
	_ = x[BC_Wall-5]
	_ = x[BC_Cyl-6]
	_ = x[BC_Neuman-7]
	_ = x[BC_Out-8]
	_ = x[BC_IVortex-9]
	_ = x[BC_Periodic-10]
}

const _BCFLAG_name = "BC_NoneBC_InBC_DirichletBC_SlipBC_FarBC_WallBC_CylBC_NeumanBC_OutBC_IVortexBC_Periodic"

var _BCFLAG_index = [...]uint8{0, 7, 12, 24, 31, 37, 44, 50, 59, 65, 75, 86}

func (i BCFLAG) String() string {
	if i >= BCFLAG(len(_BCFLAG_index)-1) {
		return "BCFLAG(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _BCFLAG_name[_BCFLAG_index[i]:_BCFLAG_index[i+1]]
}
