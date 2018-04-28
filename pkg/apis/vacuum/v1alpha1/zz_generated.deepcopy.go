// +build !ignore_autogenerated

/*
Copyright 2018 The Kubernetes Authors.

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cleaning) DeepCopyInto(out *Cleaning) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cleaning.
func (in *Cleaning) DeepCopy() *Cleaning {
	if in == nil {
		return nil
	}
	out := new(Cleaning)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Cleaning) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CleaningList) DeepCopyInto(out *CleaningList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Cleaning, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CleaningList.
func (in *CleaningList) DeepCopy() *CleaningList {
	if in == nil {
		return nil
	}
	out := new(CleaningList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CleaningList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CleaningSpec) DeepCopyInto(out *CleaningSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CleaningSpec.
func (in *CleaningSpec) DeepCopy() *CleaningSpec {
	if in == nil {
		return nil
	}
	out := new(CleaningSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CleaningStatus) DeepCopyInto(out *CleaningStatus) {
	*out = *in
	if in.Map != nil {
		in, out := &in.Map, &out.Map
		if *in == nil {
			*out = nil
		} else {
			*out = new(Map)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Path != nil {
		in, out := &in.Path, &out.Path
		*out = make([]Position, len(*in))
		copy(*out, *in)
	}
	out.Charger = in.Charger
	if in.BeginTime != nil {
		in, out := &in.BeginTime, &out.BeginTime
		if *in == nil {
			*out = nil
		} else {
			*out = (*in).DeepCopy()
		}
	}
	if in.EndTime != nil {
		in, out := &in.EndTime, &out.EndTime
		if *in == nil {
			*out = nil
		} else {
			*out = (*in).DeepCopy()
		}
	}
	if in.DayBeginTime != nil {
		in, out := &in.DayBeginTime, &out.DayBeginTime
		if *in == nil {
			*out = nil
		} else {
			*out = (*in).DeepCopy()
		}
	}
	if in.Area != nil {
		in, out := &in.Area, &out.Area
		if *in == nil {
			*out = nil
		} else {
			*out = new(int)
			**out = **in
		}
	}
	if in.Complete != nil {
		in, out := &in.Complete, &out.Complete
		if *in == nil {
			*out = nil
		} else {
			*out = new(bool)
			**out = **in
		}
	}
	if in.Code != nil {
		in, out := &in.Code, &out.Code
		if *in == nil {
			*out = nil
		} else {
			*out = new(int)
			**out = **in
		}
	}
	if in.ErrorCode != nil {
		in, out := &in.ErrorCode, &out.ErrorCode
		if *in == nil {
			*out = nil
		} else {
			*out = new(int)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CleaningStatus.
func (in *CleaningStatus) DeepCopy() *CleaningStatus {
	if in == nil {
		return nil
	}
	out := new(CleaningStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Map) DeepCopyInto(out *Map) {
	*out = *in
	if in.Data != nil {
		in, out := &in.Data, &out.Data
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Map.
func (in *Map) DeepCopy() *Map {
	if in == nil {
		return nil
	}
	out := new(Map)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Position) DeepCopyInto(out *Position) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Position.
func (in *Position) DeepCopy() *Position {
	if in == nil {
		return nil
	}
	out := new(Position)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Vacuum) DeepCopyInto(out *Vacuum) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Vacuum.
func (in *Vacuum) DeepCopy() *Vacuum {
	if in == nil {
		return nil
	}
	out := new(Vacuum)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Vacuum) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VacuumList) DeepCopyInto(out *VacuumList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Vacuum, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VacuumList.
func (in *VacuumList) DeepCopy() *VacuumList {
	if in == nil {
		return nil
	}
	out := new(VacuumList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VacuumList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VacuumSpec) DeepCopyInto(out *VacuumSpec) {
	*out = *in
	if in.Position != nil {
		in, out := &in.Position, &out.Position
		if *in == nil {
			*out = nil
		} else {
			*out = new(Position)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VacuumSpec.
func (in *VacuumSpec) DeepCopy() *VacuumSpec {
	if in == nil {
		return nil
	}
	out := new(VacuumSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VacuumStatus) DeepCopyInto(out *VacuumStatus) {
	*out = *in
	if in.Map != nil {
		in, out := &in.Map, &out.Map
		if *in == nil {
			*out = nil
		} else {
			*out = new(Map)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Position != nil {
		in, out := &in.Position, &out.Position
		if *in == nil {
			*out = nil
		} else {
			*out = new(Position)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VacuumStatus.
func (in *VacuumStatus) DeepCopy() *VacuumStatus {
	if in == nil {
		return nil
	}
	out := new(VacuumStatus)
	in.DeepCopyInto(out)
	return out
}
