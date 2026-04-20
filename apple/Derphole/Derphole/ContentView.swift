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

#Preview {
    ContentView()
}
