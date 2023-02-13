/*
 *
 * Copyright 2020 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	contextPackage      = protogen.GoImportPath("context")
	protoPackage        = protogen.GoImportPath("google.golang.org/protobuf/proto")
	protoReflectPackage = protogen.GoImportPath("google.golang.org/protobuf/reflect/protoreflect")
	errPackage          = protogen.GoImportPath("errors")
)

var (
	ctxIdent             = protogen.GoIdent{GoName: "Context", GoImportPath: contextPackage}
	protoMsgIdent        = protogen.GoIdent{GoName: "Message", GoImportPath: protoPackage}
	protoDescriptorIdent = protogen.GoIdent{GoName: "MessageDescriptor", GoImportPath: protoReflectPackage}
)

type serviceGenerateHelperInterface interface {
	generateServerFunctions(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service, serverType string, serviceDescVar string)
}

type serviceGenerateHelper struct{}

func (serviceGenerateHelper) generateServerFunctions(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service, serverType string, serviceDescVar string) {
	// Server handler implementations.
	handlerNames := make([]string, 0, len(service.Methods))
	for _, method := range service.Methods {
		hname := genServerMethod(gen, file, g, method, func(hname string) string {
			return hname
		})
		handlerNames = append(handlerNames, hname)
	}
}

var helper serviceGenerateHelperInterface = serviceGenerateHelper{}

// FileDescriptorProto.package field number
const fileDescriptorProtoPackageFieldNumber = 2

// FileDescriptorProto.syntax field number
const fileDescriptorProtoSyntaxFieldNumber = 12

// generateFile generates a _grpc.pb.go file containing gRPC service definitions.
func generateFile(gen *protogen.Plugin, file *protogen.File) *protogen.GeneratedFile {
	if len(file.Services) == 0 {
		return nil
	}
	filename := file.GeneratedFilenamePrefix + "_svcintf.pb.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)
	// Attach all comments associated with the syntax field.
	genLeadingComments(g, file.Desc.SourceLocations().ByPath(protoreflect.SourcePath{fileDescriptorProtoSyntaxFieldNumber}))
	g.P("// Code generated by protoc-gen-go-svcintf. DO NOT EDIT.")
	if file.Proto.GetOptions().GetDeprecated() {
		g.P("// ", file.Desc.Path(), " is a deprecated file.")
	} else {
		g.P("// source: ", file.Desc.Path())
	}
	g.P()
	// Attach all comments associated with the package field.
	genLeadingComments(g, file.Desc.SourceLocations().ByPath(protoreflect.SourcePath{fileDescriptorProtoPackageFieldNumber}))
	g.P("package ", file.GoPackageName)
	g.P()
	generateFileContent(gen, file, g)
	return g
}

// generateFileContent generates the gRPC service definitions, excluding the package statement.
func generateFileContent(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile) {
	if len(file.Services) == 0 {
		return
	}

	for _, service := range file.Services {
		genService(gen, file, g, service)
	}

	genFieldHelps(gen, file, g)

}

func genService(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service) {
	clientName := service.GoName + "Client"

	g.P("// ", clientName, " is the client API for ", service.GoName, " service.")

	// Client interface.
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		g.P(deprecationComment)
	}
	g.Annotate(clientName, service.Location)
	g.P("type ", clientName, " interface {")
	for _, method := range service.Methods {
		g.Annotate(clientName+"."+method.GoName, method.Location)
		if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
			g.P(deprecationComment)
		}
		g.P(method.Comments.Leading, clientSignature(g, method))
	}
	g.P("}")
	g.P()

	implName := "client_" + service.GoName
	g.P("type ", implName, " struct {")
	g.P("c ClientConn")
	g.P("defn ServiceDefn")
	g.P("}")

	var methodIndex, streamIndex int
	// Client method implementations.
	for _, method := range service.Methods {
		if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
			// Unary RPC method
			genClientMethod(gen, file, g, method, methodIndex)
			methodIndex++
		} else {
			// Streaming RPC method
			genClientMethod(gen, file, g, method, streamIndex)
			streamIndex++
		}
	}

	g.P("func New", clientName, "(c ClientConn)", clientName, "{")
	g.P("return &client_", service.GoName, "{c: c, defn: ", service.GoName, "Defn()}")
	g.P("}")
	g.P()

	// Server interface.
	serverType := service.GoName + "Server"
	g.P("// ", serverType, " is the server API for ", service.GoName, " service.")
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		g.P(deprecationComment)
	}
	g.Annotate(serverType, service.Location)
	g.P("type ", serverType, " interface {")
	for _, method := range service.Methods {
		g.Annotate(serverType+"."+method.GoName, method.Location)
		if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
			g.P(deprecationComment)
		}
		g.P(method.Comments.Leading, serverSignature(g, method))
	}
	g.P("}")
	g.P()

	// Server registration.
	serviceDescVar := service.GoName + "_ServiceDesc"

	helper.generateServerFunctions(gen, file, g, service, serverType, serviceDescVar)

	genServiceDef(gen, file, g, service)
}

func clientSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	s := method.GoName + "(ctx " + g.QualifiedGoIdent(contextPackage.Ident("Context"))
	if !method.Desc.IsStreamingClient() {
		s += ", in *" + g.QualifiedGoIdent(method.Input.GoIdent)
	}
	if !method.Desc.IsStreamingServer() {
		s += ", out *" + g.QualifiedGoIdent(method.Output.GoIdent)
	}
	s += ") ("
	if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
		s += method.Parent.GoName + "_" + method.GoName + "Client"
		s += ","

	}
	s += "error)"
	return s
}

func genClientMethod(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, method *protogen.Method, index int) {
	service := method.Parent

	isStreaming := method.Desc.IsStreamingServer() || method.Desc.IsStreamingClient()
	if isStreaming {
		//genSend := method.Desc.IsStreamingClient()
		genRecv := method.Desc.IsStreamingServer()
		//genCloseAndRecv := !method.Desc.IsStreamingServer()

		// Stream auxiliary types and methods.
		g.P("type ", service.GoName, "_", method.GoName, "Client interface {")
		if genRecv {
			g.P("Recv(*", method.Output.GoIdent, ") error")
		}
		/*
			if genSend {
				g.P("Send(*", method.Input.GoIdent, ") error")
			}

			if genCloseAndRecv {
				g.P("CloseAndRecv() (*", method.Output.GoIdent, ", error)")
			}
		*/
		g.P("}")
		g.P()
	}

	implName := "client_" + service.GoName
	g.P("func (c *", implName, ") ", clientSignature(g, method), "{")
	if !isStreaming {
		g.P("const method = \"", method.GoName, "\"")
		g.P("return c.defn.Methods[method].ClientHandler(c.c, ctx, in, out)")
	} else {
		//typ := service.GoName + "_" + method.GoName
		g.P("const method = \"", method.GoName, "\"")
		g.P("inner, err := c.defn.Methods[method].ClientStreamHandler(c.c, ctx, in)")
		g.P("if err != nil { return nil, err } ")
		g.P("return streamerImpl[*", method.Output.GoIdent, "]{c: inner}, nil")
	}
	g.P("}")
	g.P()
}

func serverSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	var reqArgs []string
	ret := "error"
	reqArgs = append(reqArgs, g.QualifiedGoIdent(contextPackage.Ident("Context")))
	if !method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, "*"+g.QualifiedGoIdent(method.Input.GoIdent))
	}
	if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
		reqArgs = append(reqArgs, method.Parent.GoName+"_"+method.GoName+"Server")
	} else {
		reqArgs = append(reqArgs, "*"+g.QualifiedGoIdent(method.Output.GoIdent))
	}
	return method.GoName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func genServerMethod(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, method *protogen.Method, hnameFuncNameFormatter func(string) string) string {
	service := method.Parent
	hname := fmt.Sprintf("_%s_%s_Handler", service.GoName, method.GoName)

	if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
		return hname
	}

	genSend := method.Desc.IsStreamingServer()
	//genSendAndClose := !method.Desc.IsStreamingServer()
	//genRecv := method.Desc.IsStreamingClient()

	// Stream auxiliary types and methods.
	typ := service.GoName + "_" + method.GoName + "Server"
	g.P("type ", typ, " interface {")
	if genSend {
		g.P("Send(m *", method.Output.GoIdent, ") error ")
	}
	/*
		if genSendAndClose {
			g.P("SendAndClose(*", method.Output.GoIdent, ") error")
		}
		if genRecv {
			g.P("Recv() (*", method.Input.GoIdent, ", error)")
		}
	*/
	g.P("}")
	g.P()

	return hname
}

func genLeadingComments(g *protogen.GeneratedFile, loc protoreflect.SourceLocation) {
	for _, s := range loc.LeadingDetachedComments {
		g.P(protogen.Comments(s))
		g.P()
	}
	if s := loc.LeadingComments; s != "" {
		g.P(protogen.Comments(s))
		g.P()
	}
}

func genServiceDef(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, svc *protogen.Service) {
	g.P("func ", svc.GoName, "Defn() ServiceDefn { ")
	g.P("return ServiceDefn {")
	g.P("Name: \"", svc.GoName, "\",")
	g.P("Methods: map[string]MethodDefn{")
	for _, meth := range svc.Methods {
		g.P("\"", meth.GoName, "\": {")
		svrTyp := meth.Parent.GoName + "Server"
		g.P("IsStreaming: ", fmt.Sprintf("%v", meth.Desc.IsStreamingServer()), ",")
		g.P("NewRequest: func() ", protoMsgIdent, " { return new(", meth.Input.GoIdent, ")},")
		g.P("NewResponse: func() ", protoMsgIdent, " { return new(", meth.Output.GoIdent, ")},")
		g.P("RequestDefn: func() ", protoDescriptorIdent, " { return new(", meth.Input.GoIdent, ").ProtoReflect().Descriptor()},")
		g.P("ResponseDefn: func() ", protoDescriptorIdent, " { return new(", meth.Output.GoIdent, ").ProtoReflect().Descriptor()},")
		g.P("Help: ", commentsToStr(meth.Comments.Leading), ",")
		if !meth.Desc.IsStreamingServer() {
			g.P("ServerHandler: func(x interface{}, ctx ", ctxIdent, ", request, response ", protoMsgIdent, ") error {")
			g.P("return x.(", svrTyp, ").", meth.GoName, "(ctx, request.(*", meth.Input.GoIdent, "), response.(*", meth.Output.GoIdent, "))")
			g.P("},")

			g.P("ClientHandler: func(conn ClientConn, ctx ", ctxIdent, ", request, response ", protoMsgIdent, ") error {")
			g.P("method := \"", svc.GoName+"."+meth.GoName, "\"")
			g.P("return conn.Request(ctx, method, request, response)")
			g.P("},")
		} else {
			g.P("ServerStreamHandler: func(x interface{}, ctx ", ctxIdent, ", request ", protoMsgIdent, ", stream ServerStream) error {")
			g.P("return x.(", svrTyp, ").", meth.GoName, "(ctx, request.(*", meth.Input.GoIdent, "), streamerImpl[*", meth.Output.GoIdent, "]{s: stream})")
			g.P("},")

			g.P("ClientStreamHandler: func(conn ClientConn, ctx ", ctxIdent, ", request ", protoMsgIdent, ") (ClientStream, error) {")
			g.P("method := \"", svc.GoName+"."+meth.GoName, "\"")
			g.P("return conn.Stream(ctx, method, request)")
			g.P("},")
		}

		g.P("},")
	}
	g.P("},")
	g.P("}")
	g.P("}")
	g.P()
}

func replaceAllStringSubmatchFunc(re *regexp.Regexp, str string, repl func([]string) string) string {
	result := ""
	lastIndex := 0

	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
		groups := []string{}
		for i := 0; i < len(v); i += 2 {
			groups = append(groups, str[v[i]:v[i+1]])
		}

		result += str[lastIndex:v[0]] + repl(groups)
		lastIndex = v[1]
	}

	return result + str[lastIndex:]
}

func commentsToStr(comments protogen.Comments) string {
	var b strings.Builder
	re := regexp.MustCompile(`([^\n])(\n)`)
	c := strings.TrimSuffix(string(comments), "\n")

	// Join lines that are separated by a single \n.
	c = replaceAllStringSubmatchFunc(re, c, func(g []string) string { return g[1] + " " })

	lines := strings.Split(c, "\n")
	l := len(lines)
	if l == 0 {
		b.WriteString("\"\"")
	}
	for i, c := range lines {
		b.WriteString("\"")
		b.WriteString(strings.TrimSpace(c))
		if i < l-1 {
			b.WriteString("\\n\"")
			b.WriteString("+")
			b.WriteString("\n")
		} else {
			b.WriteString("\"")
		}
	}
	return b.String()
}

func genFieldHelps(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile) {
	varName := "help_messages"
	g.P("var ", varName, " = map[string]map[string]string{")
	for _, msg := range file.Messages {
		g.P("\"", msg.GoIdent.GoName, "\": {")
		g.P("\"@\":")
		g.P(commentsToStr(msg.Comments.Leading), ",")
		for _, fld := range msg.Fields {
			g.P("\"", fld.Desc.Name(), "\": ")
			g.P(commentsToStr(fld.Comments.Leading), ",")
		}
		g.P("},")
	}
	g.P("}")
	g.P()
}

const deprecationComment = "// Deprecated: Do not use."
