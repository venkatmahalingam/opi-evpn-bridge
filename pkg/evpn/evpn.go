// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Intel Corporation, or its subsidiaries.

// Package evpn is the main package of the application
package evpn

import (
	"context"
	"fmt"
	"log"
	"path"

	"github.com/milosgajdos/tenus"
	pb "github.com/opiproject/opi-api/network/cloud/v1alpha1/gen/go"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"go.einride.tech/aip/resourcename"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server represents the Server object
type Server struct {
	pb.UnimplementedCloudInfraServiceServer
	Subnets    map[string]*pb.Subnet
	Interfaces map[string]*pb.Interface
}

// CreateSubnet executes the creation of the SVI/subnet
func (s *Server) CreateSubnet(_ context.Context, in *pb.CreateSubnetRequest) (*pb.Subnet, error) {
	log.Printf("CreateSubnet: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.SubnetId != "" {
		err := resourceid.ValidateUserSettable(in.SubnetId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.SubnetId, in.Subnet.Name)
		resourceID = in.SubnetId
	}
	in.Subnet.Name = fmt.Sprintf("//network.opiproject.org/vpc/%s", resourceID)
	// idempotent API when called with same key, should return same object
	snet, ok := s.Subnets[in.Subnet.Name]
	if ok {
		log.Printf("Already existing Subnet with id %v", in.Subnet.Name)
		return snet, nil
	}
	// not found, so create a new one

	// Create a new network bridge
	// It is equivalent of running: ip link add name ${ifcName} type bridge
	br, err := tenus.NewBridgeWithName("mybridge")
	if err != nil {
		log.Fatal(err)
	}

	// Bring the bridge up
	if err = br.SetLinkUp(); err != nil {
		fmt.Println(err)
	}

	// Create a dummy link
	dl, err := tenus.NewLink("mydummylink")
	if err != nil {
		log.Fatal(err)
	}

	// Add the dummy link into bridge
	if err = br.AddSlaveIfc(dl.NetInterface()); err != nil {
		log.Fatal(err)
	}

	// Bring the dummy link up
	if err = dl.SetLinkUp(); err != nil {
		fmt.Println(err)
	}
	// TODO: replace cloud -> evpn
	response := proto.Clone(in.Subnet).(*pb.Subnet)
	response.Status = &pb.SubnetStatus{HwIndex: 8}
	s.Subnets[in.Subnet.Name] = response
	log.Printf("CreateSubnet: Sending to client: %v", response)
	return response, nil
}

// DeleteSubnet deletes a SVI/subnet
func (s *Server) DeleteSubnet(_ context.Context, in *pb.DeleteSubnetRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteSubnet: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	snet, ok := s.Subnets[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	// Get an existing network bridge
	br, err := tenus.BridgeFromName("mybridge")
	if err != nil {
		log.Fatal(err)
	}
	// Bring the bridge down
	if err = br.SetLinkDown(); err != nil {
		fmt.Println(err)
	}
	// Delete link
	// $ sudo ip link delete br0 type bridge
	if err = tenus.DeleteLink("mybridge"); err != nil {
		log.Fatal(err)
	}

	delete(s.Subnets, snet.Name)
	return &emptypb.Empty{}, nil
}

// UpdateSubnet updates an SVI/subnet
func (s *Server) UpdateSubnet(_ context.Context, in *pb.UpdateSubnetRequest) (*pb.Subnet, error) {
	log.Printf("UpdateSubnet: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Subnet.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Subnets[in.Subnet.Name]
	if !ok {
		// TODO: introduce "in.AllowMissing" field. In case "true", create a new resource, don't return error
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Subnet.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.Subnet); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "UpdateSubnet method is not implemented")
}

// GetSubnet gets an SVI/subnet
func (s *Server) GetSubnet(_ context.Context, in *pb.GetSubnetRequest) (*pb.Subnet, error) {
	log.Printf("GetSubnet: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	snet, ok := s.Subnets[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO
	return &pb.Subnet{Name: in.Name, Spec: &pb.SubnetSpec{Ipv4VirtualRouterIp: snet.Spec.Ipv4VirtualRouterIp}, Status: &pb.SubnetStatus{HwIndex: 77}}, nil
}

// CreateInterface executes the creation of the interface
func (s *Server) CreateInterface(_ context.Context, in *pb.CreateInterfaceRequest) (*pb.Interface, error) {
	log.Printf("CreateInterface: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.InterfaceId != "" {
		err := resourceid.ValidateUserSettable(in.InterfaceId)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.InterfaceId, in.Interface.Name)
		resourceID = in.InterfaceId
	}
	in.Interface.Name = fmt.Sprintf("//network.opiproject.org/vpc/%s", resourceID)
	// idempotent API when called with same key, should return same object
	iface, ok := s.Interfaces[in.Interface.Name]
	if ok {
		log.Printf("Already existing Interface with id %v", in.Interface.Name)
		return iface, nil
	}
	// not found, so create a new one
	snet, ok := s.Subnets[in.Interface.Name]
	if !ok {
		// TODO: change Spec.Id.Value to bridge reference instead
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Interface.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	// Get an existing network bridge
	br, err := tenus.BridgeFromName(snet.Name)
	if err != nil {
		log.Fatal(err)
	}
	// Create a dummy link
	dl, err := tenus.NewLink("mydummylink")
	if err != nil {
		log.Fatal(err)
	}
	// Add the dummy link into bridge
	if err = br.AddSlaveIfc(dl.NetInterface()); err != nil {
		log.Fatal(err)
	}

	// Bring the dummy link up
	if err = dl.SetLinkUp(); err != nil {
		fmt.Println(err)
	}
	// TODO: replace cloud -> evpn
	response := proto.Clone(in.Interface).(*pb.Interface)
	response.Status = &pb.InterfaceStatus{IfIndex: 8}
	s.Interfaces[in.Interface.Name] = response
	log.Printf("CreateInterface: Sending to client: %v", response)
	return response, nil
}

// DeleteInterface deletes an interface
func (s *Server) DeleteInterface(_ context.Context, in *pb.DeleteInterfaceRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteInterface: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	iface, ok := s.Interfaces[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}

	// Delete link
	// $ sudo ip link delete br0 type bridge
	if err := tenus.DeleteLink(iface.Name); err != nil {
		log.Fatal(err)
	}

	delete(s.Interfaces, iface.Name)
	return &emptypb.Empty{}, nil
}

// UpdateInterface updates an Nvme Subsystem
func (s *Server) UpdateInterface(_ context.Context, in *pb.UpdateInterfaceRequest) (*pb.Interface, error) {
	log.Printf("UpdateInterface: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Interface.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Interfaces[in.Interface.Name]
	if !ok {
		// TODO: introduce "in.AllowMissing" field. In case "true", create a new resource, don't return error
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Interface.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.Interface); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "UpdateInterface method is not implemented")
}

// GetInterface gets an Interface
func (s *Server) GetInterface(_ context.Context, in *pb.GetInterfaceRequest) (*pb.Interface, error) {
	log.Printf("GetInterface: Received from client: %v", in)
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// Validate that a resource name conforms to the restrictions outlined in AIP-122.
	if err := resourcename.Validate(in.Name); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// fetch object from the database
	snet, ok := s.Interfaces[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO
	return &pb.Interface{Name: in.Name, Spec: &pb.InterfaceSpec{Ifid: snet.Spec.Ifid}, Status: &pb.InterfaceStatus{IfIndex: 55}}, nil
}
