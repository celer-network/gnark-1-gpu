/*
Copyright © 2020 ConsenSys

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package eddsa provides a ZKP-circuit function to verify a EdDSA signature.
package eddsa

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/twistededwards"
	"github.com/consensys/gnark/std/hash/mimc"
)

// PublicKey stores an eddsa public key (to be used in gnark circuit)
type PublicKey struct {
	A     twistededwards.Point
	Curve twistededwards.EdCurve
}

// Signature stores a signature  (to be used in gnark circuit)
// An EdDSA signature is a tuple (R,S) where R is a point on the twisted Edwards curve
// and S a scalar. S can be greater than r, the size of the zk snark field, and must
// not be reduced modulo r. Therefore it is split in S1 and S2, such that if r is n-bits long,
// S = 2^(n/2)*S1 + S2. In other words, S is written S1S2 in basis 2^(n/2).
type Signature struct {
	R twistededwards.Point
	//S1, S2 frontend.Variable
	S frontend.Variable
}

// Verify verifies an eddsa signature
// cf https://en.wikipedia.org/wiki/EdDSA
func Verify(cs *frontend.ConstraintSystem, sig Signature, msg frontend.Variable, pubKey PublicKey) error {

	// compute H(R, A, M), all parameters in data are in Montgomery form
	data := []frontend.Variable{
		sig.R.X,
		sig.R.Y,
		pubKey.A.X,
		pubKey.A.Y,
		msg,
	}

	hash, err := mimc.NewMiMC("seed", pubKey.Curve.ID, cs)
	if err != nil {
		return err
	}
	hash.Write(data...)
	//hramConstant := hash.Sum(data...)
	hramConstant := hash.Sum()

	// lhs = cofactor*SB
	cofactor := pubKey.Curve.Cofactor.Uint64()
	lhs := twistededwards.Point{}

	// [cofactor*(2^basis*S1 +  S2)]G
	lhs.ScalarMulFixedBase(cs, pubKey.Curve.BaseX, pubKey.Curve.BaseY, sig.S, pubKey.Curve)

	switch cofactor {
	case 4:
		lhs.Double(cs, &lhs, pubKey.Curve).
			Double(cs, &lhs, pubKey.Curve)
	case 8:
		lhs.Double(cs, &lhs, pubKey.Curve).
			Double(cs, &lhs, pubKey.Curve).Double(cs, &lhs, pubKey.Curve)
	}

	lhs.MustBeOnCurve(cs, pubKey.Curve)

	//rhs = cofactor*(R+H(R,A,M)*A)
	rhs := twistededwards.Point{}
	rhs.ScalarMulNonFixedBase(cs, &pubKey.A, hramConstant, pubKey.Curve).
		AddGeneric(cs, &rhs, &sig.R, pubKey.Curve)
	switch cofactor {
	case 4:
		rhs.Double(cs, &rhs, pubKey.Curve).
			Double(cs, &rhs, pubKey.Curve)
	case 8:
		rhs.Double(cs, &rhs, pubKey.Curve).
			Double(cs, &rhs, pubKey.Curve).Double(cs, &rhs, pubKey.Curve)
	}

	rhs.MustBeOnCurve(cs, pubKey.Curve)

	cs.AssertIsEqual(lhs.X, rhs.X)
	cs.AssertIsEqual(lhs.Y, rhs.Y)

	return nil
}
