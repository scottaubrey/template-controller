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

package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"

	"github.com/google/go-github/v47/github"
	"golang.org/x/oauth2"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	templatesv1alpha1 "github.com/kluctl/template-controller/api/v1alpha1"
)

// ListGithubDeploymentsReconciler reconciles a ListGithubDeployments object
type ListGithubDeploymentsReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	FieldManager string
}

//+kubebuilder:rbac:groups=templates.kluctl.io,resources=listgithubdeployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=templates.kluctl.io,resources=listgithubdeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=templates.kluctl.io,resources=listgithubdeployments/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ListGithubDeploymentsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var obj templatesv1alpha1.ListGithubDeployments
	err := r.Get(ctx, req.NamespacedName, &obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.doReconcile(ctx, &obj)
	if err != nil {
		c := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: obj.GetGeneration(),
			Reason:             "Error",
			Message:            err.Error(),
		}
		apimeta.SetStatusCondition(&obj.Status.Conditions, c)
	} else {
		c := metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: obj.GetGeneration(),
			Reason:             "Success",
			Message:            "Success",
		}
		apimeta.SetStatusCondition(&obj.Status.Conditions, c)
	}

	// TODO optimize the update as it currently causes to update all deployments on every call
	// patching is not working very well as causes nulls to be pruned and full array replacement for every single change
	err = r.Status().Update(ctx, &obj, SubResourceFieldOwner(r.FieldManager))
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{
		RequeueAfter: obj.Spec.Interval.Duration,
	}, nil
}

func (r *ListGithubDeploymentsReconciler) doReconcile(ctx context.Context, obj *templatesv1alpha1.ListGithubDeployments) error {
	var token string
	var err error

	if obj.Spec.TokenRef != nil {
		token, err = GetSecretToken(ctx, r.Client, obj.Namespace, *obj.Spec.TokenRef)
		if err != nil {
			return err
		}
	}

	var tc *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc = oauth2.NewClient(ctx, ts)
	}
	gh := github.NewClient(tc)

	listOpts := &github.DeploymentsListOptions{}
	listOpts.Environment = *obj.Spec.Environment
	listOpts.SHA = *obj.Spec.Sha
	listOpts.Ref = *obj.Spec.Ref
	listOpts.Task = *obj.Spec.Task
	listOpts.Page = 1
	listOpts.PerPage = 100

	var result []*github.Deployment
	for true {
		if len(result)+listOpts.PerPage > obj.Spec.Limit {
			listOpts.PerPage = obj.Spec.Limit - len(result)
		}

		page, _, err := gh.Repositories.ListDeployments(ctx, obj.Spec.Owner, obj.Spec.Repo, listOpts)

		if err != nil {
			return err
		}
		result = append(result, page...)
		if len(page) != listOpts.PerPage || len(result) >= obj.Spec.Limit {
			break
		}
		listOpts.Page += 1
	}

	sort.Slice(result, func(i, j int) bool {
		return *result[i].ID < *result[j].ID
	})

	newDeployments := make([]runtime.RawExtension, 0, len(result))

	for _, pr := range result {
		err := r.simplifyObject(reflect.ValueOf(pr))
		if err != nil {
			return err
		}

		j, err := json.Marshal(pr)
		if err != nil {
			return err
		}

		newDeployments = append(newDeployments, runtime.RawExtension{Raw: j})
	}

	obj.Status.Deployments = newDeployments

	return nil
}

func (r *ListGithubDeploymentsReconciler) simplifyObject(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Pointer:
		return r.simplifyObject(v.Elem())
	case reflect.Struct:
		fv := v.Addr().Interface()
		switch x := fv.(type) {
		case *github.PullRequest:
			return r.simplifyPullRequest(x)
		case *github.User:
			return r.simplifyUser(x)
		case *github.Organization:
			return r.simplifyOrganisation(x)
		case *github.Repository:
			return r.simplifyRepository(x)
		default:
			return r.simplifyObjectGeneric(v)
		}
	case reflect.Slice:
		l := v.Len()
		for i := 0; i < l; i++ {
			x := v.Index(i)
			err := r.simplifyObject(x)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ListGithubDeploymentsReconciler) simplifyObjectGeneric(v reflect.Value) error {
	v = reflect.Indirect(v)
	for _, field := range reflect.VisibleFields(v.Type()) {
		f := v.FieldByIndex(field.Index)
		if f.IsZero() {
			continue
		}
		err := r.simplifyObject(f)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ListGithubDeploymentsReconciler) simplifyPullRequest(x *github.PullRequest) error {
	x.Links = nil
	return r.simplifyObjectGeneric(reflect.ValueOf(x))
}

func (r *ListGithubDeploymentsReconciler) simplifyUser(x *github.User) error {
	if x == nil {
		return nil
	}
	*x = github.User{
		ID:    x.ID,
		Login: x.Login,
	}
	return nil
}

func (r *ListGithubDeploymentsReconciler) simplifyOrganisation(x *github.Organization) error {
	if x == nil {
		return nil
	}
	*x = github.Organization{
		ID:    x.ID,
		Login: x.Login,
	}
	return nil
}

func (r *ListGithubDeploymentsReconciler) simplifyRepository(x *github.Repository) error {
	if x == nil {
		return nil
	}
	*x = github.Repository{
		ID:       x.ID,
		Owner:    x.Owner,
		Name:     x.Name,
		FullName: x.FullName,
	}
	return r.simplifyObjectGeneric(reflect.ValueOf(x))
}

// SetupWithManager sets up the controller with the Manager.
func (r *ListGithubDeploymentsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&templatesv1alpha1.ListGithubDeployments{}).
		Complete(r)
}
