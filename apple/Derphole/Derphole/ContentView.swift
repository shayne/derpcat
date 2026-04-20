//
//  ContentView.swift
//  Derphole
//
//  Created by Shayne Sweeney on 4/19/26.
//

import SwiftUI

struct ContentView: View {
    @StateObject private var tokenStore = TokenStore()

    var body: some View {
        TabView {
            NavigationStack {
                FilesTabView()
            }
            .tabItem {
                Label("Files", systemImage: "doc")
            }
            .tag(AppTab.files)

            NavigationStack {
                WebTabView(tokenStore: tokenStore)
            }
            .tabItem {
                Label("Web", systemImage: "safari")
            }
            .tag(AppTab.web)

            NavigationStack {
                SSHTabView(tokenStore: tokenStore)
            }
            .tabItem {
                Label("SSH", systemImage: "terminal")
            }
            .tag(AppTab.ssh)
        }
    }
}

private struct WebTabView: View {
    @ObservedObject var tokenStore: TokenStore

    var body: some View {
        VStack(spacing: 16) {
            Spacer()
            Label("Web tunnel support is next.", systemImage: "safari")
                .font(.headline)
            if let webToken = tokenStore.webToken {
                Text(TransferFormatting.fingerprint(webToken))
                    .font(.caption.monospaced())
                    .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(20)
        .navigationTitle("Web")
        .accessibilityIdentifier("webTab")
    }
}

private struct SSHTabView: View {
    @ObservedObject var tokenStore: TokenStore

    var body: some View {
        VStack(spacing: 16) {
            Spacer()
            Label("SSH tunnel support is next.", systemImage: "terminal")
                .font(.headline)
            if let tcpToken = tokenStore.tcpToken {
                Text(TransferFormatting.fingerprint(tcpToken))
                    .font(.caption.monospaced())
                    .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(20)
        .navigationTitle("SSH")
        .accessibilityIdentifier("sshTab")
    }
}

#Preview {
    ContentView()
}
