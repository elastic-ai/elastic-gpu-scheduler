/*
Copyright 2022.

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
// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "elasticgpu.io/elastic-gpu/api/elasticgpu.io/v1alpha1"
	scheme "elasticgpu.io/elastic-gpu/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ElasticGPUsGetter has a method to return a ElasticGPUInterface.
// A group's client should implement this interface.
type ElasticGPUsGetter interface {
	ElasticGPUs(namespace string) ElasticGPUInterface
}

// ElasticGPUInterface has methods to work with ElasticGPU resources.
type ElasticGPUInterface interface {
	Create(ctx context.Context, elasticGPU *v1alpha1.ElasticGPU, opts v1.CreateOptions) (*v1alpha1.ElasticGPU, error)
	Update(ctx context.Context, elasticGPU *v1alpha1.ElasticGPU, opts v1.UpdateOptions) (*v1alpha1.ElasticGPU, error)
	UpdateStatus(ctx context.Context, elasticGPU *v1alpha1.ElasticGPU, opts v1.UpdateOptions) (*v1alpha1.ElasticGPU, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ElasticGPU, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ElasticGPUList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ElasticGPU, err error)
	ElasticGPUExpansion
}

// elasticGPUs implements ElasticGPUInterface
type elasticGPUs struct {
	client rest.Interface
	ns     string
}

// newElasticGPUs returns a ElasticGPUs
func newElasticGPUs(c *ElasticgpuV1alpha1Client, namespace string) *elasticGPUs {
	return &elasticGPUs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the elasticGPU, and returns the corresponding elasticGPU object, and an error if there is any.
func (c *elasticGPUs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ElasticGPU, err error) {
	result = &v1alpha1.ElasticGPU{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("elasticgpus").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ElasticGPUs that match those selectors.
func (c *elasticGPUs) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ElasticGPUList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ElasticGPUList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("elasticgpus").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested elasticGPUs.
func (c *elasticGPUs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("elasticgpus").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a elasticGPU and creates it.  Returns the server's representation of the elasticGPU, and an error, if there is any.
func (c *elasticGPUs) Create(ctx context.Context, elasticGPU *v1alpha1.ElasticGPU, opts v1.CreateOptions) (result *v1alpha1.ElasticGPU, err error) {
	result = &v1alpha1.ElasticGPU{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("elasticgpus").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(elasticGPU).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a elasticGPU and updates it. Returns the server's representation of the elasticGPU, and an error, if there is any.
func (c *elasticGPUs) Update(ctx context.Context, elasticGPU *v1alpha1.ElasticGPU, opts v1.UpdateOptions) (result *v1alpha1.ElasticGPU, err error) {
	result = &v1alpha1.ElasticGPU{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("elasticgpus").
		Name(elasticGPU.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(elasticGPU).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *elasticGPUs) UpdateStatus(ctx context.Context, elasticGPU *v1alpha1.ElasticGPU, opts v1.UpdateOptions) (result *v1alpha1.ElasticGPU, err error) {
	result = &v1alpha1.ElasticGPU{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("elasticgpus").
		Name(elasticGPU.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(elasticGPU).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the elasticGPU and deletes it. Returns an error if one occurs.
func (c *elasticGPUs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("elasticgpus").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *elasticGPUs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("elasticgpus").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched elasticGPU.
func (c *elasticGPUs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ElasticGPU, err error) {
	result = &v1alpha1.ElasticGPU{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("elasticgpus").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
